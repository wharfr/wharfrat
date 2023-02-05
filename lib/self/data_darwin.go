//go:build !bootstrap

package self

import _ "embed"

//go:embed dist/wr-linux
var linuxData []byte

