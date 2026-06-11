// Package agentrun drives kairos-agent's manual-install and parses its
// JSON-Lines progress stream. It has no TUI dependencies so installer
// frontends (and tests) can reuse it in isolation.
//
// It implements the installer side of the kairos-agent installer contract:
// build a `kairos-agent manual-install` invocation, run it with progress
// emission enabled, and turn the agent's JSON-Lines stdout into structured
// progress events.
package agentrun

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
)

// EnvAgentBin overrides agent discovery with an explicit path.
const EnvAgentBin = "KAIROS_AGENT_BIN"

// Contract vocabulary — the JSON-Lines progress protocol that kairos-agent
// emits and installer frontends consume. Both sides should reference these
// constants instead of hard-coding the strings.
const (
	// EnvProgress, when set to a non-empty value in the agent's environment,
	// makes kairos-agent emit progress events on stdout.
	EnvProgress = "KAIROS_AGENT_PROGRESS"

	// Event values for ProgressEvent.Event.
	EventStep  = "step"
	EventError = "error"
)

// Step values for ProgressEvent.Step, in the order the agent emits them.
const (
	StepPartition     = "partition"
	StepBeforeInstall = "before-install"
	StepActive        = "active"
	StepBootloader    = "bootloader"
	StepRecovery      = "recovery"
	StepPassive       = "passive"
	StepAfterInstall  = "after-install"
	StepDone          = "done"
)

// Steps lists the step events in the order the agent emits them on a full,
// successful install. Steps that do not run (e.g. partition on a NoFormat
// install) are simply omitted from the stream.
var Steps = []string{
	StepPartition,
	StepBeforeInstall,
	StepActive,
	StepBootloader,
	StepRecovery,
	StepPassive,
	StepAfterInstall,
	StepDone,
}

// agentBinName is the fixed name looked up on PATH.
const agentBinName = "kairos-agent"

// ProgressEvent is one parsed JSON-Lines progress line from the agent.
type ProgressEvent struct {
	Event   string `json:"event"`
	Step    string `json:"step"`
	Message string `json:"message"`
}

// ResolveAgentBin returns the kairos-agent path: KAIROS_AGENT_BIN (must exist)
// then kairos-agent on PATH, else "".
func ResolveAgentBin() string {
	if p := os.Getenv(EnvAgentBin); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(agentBinName); err == nil {
		return p
	}
	return ""
}

// Command builds the manual-install invocation. finishAction is one of
// "reboot", "poweroff", or anything else (no finish flag). It sets
// KAIROS_AGENT_PROGRESS=1 so the agent emits progress events.
func Command(agentBin, cfgPath, source, finishAction string) *exec.Cmd {
	args := []string{"manual-install"}
	if source != "" {
		args = append(args, "--source", source)
	}
	args = append(args, "--use-default-dirs")
	switch finishAction {
	case "reboot":
		args = append(args, "--reboot")
	case "poweroff":
		args = append(args, "--poweroff")
	}
	args = append(args, cfgPath)

	cmd := exec.Command(agentBin, args...)
	cmd.Env = append(os.Environ(), EnvProgress+"=1")
	return cmd
}

// ParseLine parses one stdout line. ok is true only for a JSON object carrying
// a non-empty "event" field; everything else (plain logs, eventless JSON) is
// reported as ok=false.
func ParseLine(line []byte) (ProgressEvent, bool) {
	var ev ProgressEvent
	if err := json.Unmarshal(line, &ev); err != nil {
		return ProgressEvent{}, false
	}
	if ev.Event == "" {
		return ProgressEvent{}, false
	}
	return ev, true
}

// Run execs the agent, calling onEvent for each progress event and onLog for
// each non-event stdout line. It returns the process exit error, if any.
func Run(agentBin, cfgPath, source, finishAction string, onEvent func(ProgressEvent), onLog func(string)) error {
	cmd := Command(agentBin, cfgPath, source, finishAction)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if ev, ok := ParseLine(line); ok {
			onEvent(ev)
		} else if len(line) > 0 {
			onLog(string(line))
		}
	}
	return cmd.Wait()
}
