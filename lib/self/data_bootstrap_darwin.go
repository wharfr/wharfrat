//go:build bootstrap

package self

import _ "embed"

//go:embed dist/bootstrap/wr-linux
var linuxData []byte

