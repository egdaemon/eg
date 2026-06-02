#!/bin/bash

# loadenv sources an environment file
# usage: loadenv foo.env
# file format: https://www.freedesktop.org/software/systemd/man/environment.d.html
function loadenv() {
	set -a
	local i="${1}"
	source ${i}
	set +a
}

# loadenvdir sources every file within a directory. (not recursive)
# usage: loadenvdir mydir.d
# file format: https://www.freedesktop.org/software/systemd/man/environment.d.html
function loadenvdir() {
	set -a
	local dir="${1}"
	if [ -d "${dir}" ]; then
		for i in ${dir}/*
		do
			source ${i}
		done
	fi
	set +a
}
