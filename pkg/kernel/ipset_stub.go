//go:build !linux

package kernel

type SetInfo struct {
	Name     string
	TypeName string
	Entries  uint32
}

func LoadedSets() (map[string]SetInfo, error) {
	return map[string]SetInfo{}, nil
}

func ApplyIfLoaded(name, hash string, lines []string) (bool, error) {
	return false, nil
}
