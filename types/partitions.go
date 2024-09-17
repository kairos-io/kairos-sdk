package types

type Partition struct {
	Name            string   `yaml:"-"`
	FilesystemLabel string   `yaml:"label,omitempty" mapstructure:"label"`
	Size            uint     `yaml:"size,omitempty" mapstructure:"size"`
	FS              string   `yaml:"fs,omitempty" mapstrcuture:"fs"`
	Flags           []string `yaml:"flags,omitempty" mapstrcuture:"flags"`
	UUID            string   `yaml:"uuid,omitempty" mapstructure:"uuid"`
	MountPoint      string   `yaml:"-"`
	Path            string   `yaml:"-"`
	Disk            string   `yaml:"-"`
}

type PartitionList []*Partition

type Disk struct {
	Name       string        `json:"name,omitempty" yaml:"name,omitempty"`
	SizeBytes  uint64        `json:"size_bytes,omitempty" yaml:"size_bytes,omitempty"`
	UUID       string        `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Partitions PartitionList `json:"partitions,omitempty" yaml:"partitions,omitempty"`
}
