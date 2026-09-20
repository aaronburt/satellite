package updater

import (
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
	Patch int
}

func ParseVersion(v string) Version {
	clean := strings.TrimSpace(v)
	clean = strings.TrimPrefix(clean, "v")
	clean = strings.TrimPrefix(clean, "V")

	if idx := strings.IndexAny(clean, "-+"); idx != -1 {
		clean = clean[:idx]
	}

	parts := strings.Split(clean, ".")
	var ver Version

	if len(parts) > 0 {
		ver.Major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		ver.Minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) > 2 {
		ver.Patch, _ = strconv.Atoi(parts[2])
	}

	return ver
}

func CompareVersions(v1, v2 string) int {
	ver1 := ParseVersion(v1)
	ver2 := ParseVersion(v2)

	if ver1.Major != ver2.Major {
		if ver1.Major > ver2.Major {
			return 1
		}
		return -1
	}

	if ver1.Minor != ver2.Minor {
		if ver1.Minor > ver2.Minor {
			return 1
		}
		return -1
	}

	if ver1.Patch != ver2.Patch {
		if ver1.Patch > ver2.Patch {
			return 1
		}
		return -1
	}

	return 0
}

func IsNewer(current, candidate string) bool {
	return CompareVersions(candidate, current) > 0
}
