package e2etest

import "os"

func BaseDir(pattern string) (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, pattern)
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}
