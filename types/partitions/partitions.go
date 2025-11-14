package partitions

type Partition struct {
	Name            string   `yaml:"name,omitempty" mapstructure:"name" json:"name,omitempty"`
	FilesystemLabel string   `yaml:"label,omitempty" mapstructure:"label" json:"label,omitempty"`
	Size            uint     `yaml:"size,omitempty" mapstructure:"size" json:"size,omitempty"`
	FS              string   `yaml:"fs,omitempty" mapstrcuture:"fs" json:"fs,omitempty"`
	Flags           []string `yaml:"flags,omitempty" mapstrcuture:"flags" json:"flags,omitempty"`
	UUID            string   `yaml:"uuid,omitempty" mapstructure:"uuid" json:"uuid,omitempty"`
	MountPoint      string   `yaml:"-" json:"-"` // MountPoint is not serialized
	LoopDevice      string   `yaml:"-" json:"-"` // LoopDevice is not serialized
	Path            string   `yaml:"-" json:"-"` // Path is not serialized
	Disk            string   `yaml:"-" json:"-"` // Disk is not serialized
}

type PartitionList []*Partition

type Disk struct {
	Name       string        `json:"name,omitempty" mapstructure:"name" yaml:"name,omitempty"`
	SizeBytes  uint64        `json:"size_bytes,omitempty" mapstructure:"size_bytes" yaml:"size_bytes,omitempty"`
	UUID       string        `json:"uuid,omitempty" mapstructure:"uuid" yaml:"uuid,omitempty"`
	Partitions PartitionList `json:"partitions,omitempty" mapstructure:"partitions" yaml:"partitions,omitempty"`
}

type ElementalPartitions struct {
	BIOS       *Partition `yaml:"-" json:"-"` // We dont want marshal/unmarshal this field
	EFI        *Partition `yaml:"-" json:"-"` // We dont want marshal/unmarshal this field
	OEM        *Partition `yaml:"oem,omitempty" mapstructure:"oem" json:"oem,omitempty"`
	Recovery   *Partition `yaml:"recovery,omitempty" mapstructure:"recovery" json:"recovery,omitempty"`
	State      *Partition `yaml:"state,omitempty" mapstructure:"state" json:"state,omitempty"`
	Persistent *Partition `yaml:"persistent,omitempty" mapstructure:"persistent" json:"persistent,omitempty"`
}
