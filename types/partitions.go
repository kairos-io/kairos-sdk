package types

type Partition struct {
	Name            string   `yaml:"-"`
	FilesystemLabel string   `yaml:"label,omitempty" mapstructure:"label"`
	Size            uint     `yaml:"size,omitempty" mapstructure:"size"`
	FS              string   `yaml:"fs,omitempty" mapstrcuture:"fs"`
	Flags           []string `yaml:"flags,omitempty" mapstrcuture:"flags"`
	Type            string   `yaml:"type,omitempty" mapstrcuture:"type"`
	UUID            string
	MountPoint      string `yaml:"-"`
	Path            string `yaml:"-"`
	Disk            string `yaml:"-"`
}

type PartitionList []*Partition
