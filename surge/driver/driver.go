package driver

type Driver string

const (
	DriverRedis  Driver = "redis"
	DriverMemory Driver = "memory"
	DriverCustom Driver = "custom"
)
