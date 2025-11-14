package bundles

type Bundles []Bundle

type Bundle struct {
	Repository string   `yaml:"repository,omitempty" json:"repository,omitempty" mapstructure:"repository"`
	Rootfs     string   `yaml:"rootfs_path,omitempty" json:"rootfs_path,omitempty" mapstructure:"rootfs_path"`
	DB         string   `yaml:"db_path,omitempty" json:"db_path,omitempty" mapstructure:"db_path"`
	LocalFile  bool     `yaml:"local_file,omitempty" json:"local_file,omitempty" mapstructure:"local_file"`
	Targets    []string `yaml:"targets,omitempty" json:"targets,omitempty" mapstructure:"targets"`
}
