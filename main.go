package main

import (
	"fmt"
	"github.com/kairos-io/kairos-sdk/loop"
	"github.com/kairos-io/kairos-sdk/types"
)

func main() {
	l := types.NewKairosLogger("test", "debug", false)
	loopDevice, err := loop.Loop("disk.img", true, l)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer loop.Unloop(loopDevice, l)
	devs, err := loop.CreateMappingsFromImage(loopDevice)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Created GPT partitions:")
		for _, d := range devs {
			fmt.Println(" -", d)
		}
	}
}
