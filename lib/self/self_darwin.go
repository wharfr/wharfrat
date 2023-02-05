package self

func GetLinux() ([]byte, error) {
	return linuxData, nil
}

var HomeMount = []string{
	"/Users:/home",
	"/Users:/Users",
}
