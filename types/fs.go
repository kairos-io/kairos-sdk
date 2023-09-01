package types

// KairosFS is our interface for methods that need an FS
// We should try to keep it to a minimum so we can change between backends easily if needed
type KairosFS interface {
	ReadFile(filename string) ([]byte, error)
}
