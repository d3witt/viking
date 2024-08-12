package config

type Profile struct {
	Email string
}

const defaultProfileName = "default"

func (c Config) GetDefaultProfile() Profile {
	return c.Profiles[defaultProfileName]
}

func (c *Config) SetDefaultProfile(profile Profile) error {
	c.Profiles[defaultProfileName] = profile

	return c.Save()
}
