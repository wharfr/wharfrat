package venv

import (
	"os"
	"path/filepath"
	"text/template"
)

const activateScript = `
wr-deactivate() {
    if ! [ -z "${_OLD_WRENV_PATH+_}" ]; then
        PATH="$_OLD_WRENV_PATH"
        export PATH
        unset _OLD_WRENV_PATH
    fi

    if [ -n "${BASH_VERSION-}" ]; then
        hash -r
    fi

    if ! [ -z "${_OLD_WRENV_PS1+_}" ]; then
        PS1="$_OLD_WRENV_PS1"
        export PS1
        unset _OLD_WRENV_PS1
    fi

    unset WHARFRAT_ENV

    if [ "$1" != "nosuicide" ]; then
        unset -f wr-deactivate
    fi
}

# cleanup
wr-deactivate nosuicide

WHARFRAT_ENV="{{ .Path }}"
export WHARFRAT_ENV

_OLD_WRENV_PATH="$PATH"
PATH="$WHARFRAT_ENV/bin:$PATH"
export PATH

if [ -z "${WHARFRAT_ENV_DISABLE_PROMPT-}" ]; then
	_OLD_WRENV_PS1="$PS1"
	PS1="(wr:$(basename "$WHARFRAT_ENV")) $PS1"
	export PS1
fi

if [ -n "${BASH_VERSION-}" ]; then
    hash -r
fi

if [ -z "${WHARFRAT_ENV_QUIET-}" ]; then
    echo "Activated wharfrat environment, 'wr-deactivate' to deactivate."
fi
`

func writeActivate(path string) error {
	t, err := template.New("activate").Parse(activateScript)
	if err != nil {
		return err
	}

	filename := filepath.Join(path, "bin", "activate")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := t.Execute(f, map[string]interface{}{
		"Path": path,
	}); err != nil {
		return err
	}

	return nil
}