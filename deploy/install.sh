#!/bin/sh
set -eu

version=unstamped
sha256_amd64=unstamped
sha256_arm64=unstamped
downloads=https://github.com/gopherium/gophenberg/releases/download
protocols='=https'
stall_seconds=60
release_key=MCowBQYDK2VwAyEANLMkB0RToFiEfIMw2VdG5Ml636MuXEwm4C+hGbbd5Mw=
standby_key=MCowBQYDK2VwAyEAWrchu9WREyEOuKPjK7hoKSyEClGejpYjcaISqSJ1CPM=
installer_folder=/var/lib/gophenberg-installer
root_id=0

# say prints its arguments as one line on standard output.
say() {
	printf '%s\n' "$*"
}

# stop prints why install.sh stops on standard error and ends it with exit code 1.
stop() {
	printf 'install.sh: %s\n' "$*" >&2
	exit 1
}

# refuse_archive stops when an argument names -archive, since install.sh passes the archive it checked itself.
refuse_archive() {
	for argument in "$@"; do
		case "$argument" in
		-archive | --archive | -archive=* | --archive=*)
			stop "install.sh passes the archive it checked itself, so leave out -archive"
			;;
		esac
	done
}

# pick_platform sets the archive's processor name and its stamped checksum from this server's machine name.
pick_platform() {
	machine=$(uname -m)
	case "$machine" in
	x86_64)
		arch=amd64
		wanted=$sha256_amd64
		;;
	aarch64)
		arch=arm64
		wanted=$sha256_arm64
		;;
	*)
		stop "Gophenberg ships for x86_64 and aarch64 servers, and this server is $machine"
		;;
	esac
}

# need stops unless the tool is on the path, naming the package that adds it.
need() {
	command -v "$1" > /dev/null 2>&1 || stop "install.sh needs $1, so add it with sudo apt-get install $1"
}

# check_openssl stops unless openssl is OpenSSL 3 or newer, which checking the signature needs.
check_openssl() {
	openssl_line=$(openssl version)
	openssl_rest=${openssl_line#* }
	openssl_found="${openssl_line%% *} ${openssl_rest%% *}"
	case "$openssl_found" in
	"OpenSSL "[3-9].* | "OpenSSL "[1-9][0-9]*) ;;
	*) stop "checking the download needs OpenSSL 3 or newer, and this server has $openssl_found" ;;
	esac
}

# make_work makes the installer folder for root alone when missing, checks it, and makes a download folder in it.
make_work() {
	if [ ! -e "$installer_folder" ] && [ ! -L "$installer_folder" ]; then
		mkdir -m 0700 "$installer_folder"
	fi
	if [ -z "$(find "$installer_folder" -prune -type d -user "$root_id" ! -perm -020 ! -perm -002 -print)" ]; then
		stop "$installer_folder, which holds the install record, must be a real folder owned by root" \
			"and writable by root alone, so nothing was installed"
	fi
	work=
	trap '[ -z "$work" ] || rm -rf "$work"' EXIT
	trap 'exit 129' HUP
	trap 'exit 130' INT
	trap 'exit 143' TERM
	work=$(mktemp -d "$installer_folder/download.XXXXXXXXXX")
}

# fetch downloads one file of the release into the download folder over HTTPS, or stops naming its address.
fetch() {
	address="$downloads/v$version/$1"
	code=0
	curl -fsSL --proto "$protocols" --proto-redir "$protocols" --tlsv1.2 --speed-limit 1 --speed-time "$stall_seconds" \
		-o "$work/$1" "$address" || code=$?
	[ "$code" != 0 ] || return 0
	unreached="cannot download $address, so nothing was installed"
	case "$code" in
	6 | 7) stop "$unreached. GitHub's downloads need IPv4, which a server with IPv6 alone cannot reach" ;;
	esac
	stop "$unreached"
}

# check_sum stops unless the archive's SHA-256 is the one stamped for this server's processor.
check_sum() {
	sum=$(openssl dgst -sha256 -r "$work/$name")
	[ "${sum%% *}" = "$wanted" ] ||
		stop "the archive is not the one this install.sh names, so nothing was installed"
}

# signed_by reports whether the key, the one base64 line of its PEM, signed the archive's digest.
signed_by() {
	printf '%s\n' '-----BEGIN PUBLIC KEY-----' "$1" '-----END PUBLIC KEY-----' > "$work/key.pem"
	openssl pkeyutl -verify -pubin -inkey "$work/key.pem" -rawin \
		-in "$work/digest.bin" -sigfile "$work/signature.bin" > /dev/null 2>&1
}

# check_signature stops unless the release key or the standby made the archive's .sig.
check_signature() {
	unsigned="the archive's signature matches neither Gophenberg release key, so nothing was installed"
	[ -s "$work/$name.sig" ] || stop "$unsigned"
	line=$(tr '_-' '/+' < "$work/$name.sig")
	line=$(printf '%s' "$line" | tr -d '\n')
	case $((${#line} % 4)) in
	2) line="$line==" ;;
	3) line="$line=" ;;
	esac
	printf '%s' "$line" > "$work/signature.b64"
	openssl base64 -d -A -in "$work/signature.b64" -out "$work/signature.bin" || stop "$unsigned"
	openssl dgst -sha384 -binary -out "$work/digest.bin" "$work/$name"
	signed_by "$release_key" || signed_by "$standby_key" || stop "$unsigned"
}

# hand_over takes gophenberg out of the checked archive and runs its server:install from /, ending with its code.
hand_over() {
	tar -xzf "$work/$name" -C "$work" gophenberg ||
		stop "cannot unpack gophenberg from $name, so nothing was installed"
	cd /
	code=0
	"$work/gophenberg" server:install -archive "$work/$name" "$@" < /dev/null || code=$?
	exit "$code"
}

# main installs Gophenberg on this Linux server, as https://docs.gophenberg.org/self-hosting/server-install/ explains.
main() {
	[ "$#" = "$arguments_given" ] || stop "this copy of install.sh was cut off, so nothing was installed"
	PATH="$PATH:/usr/sbin:/sbin"
	[ "$(id -u)" = "$root_id" ] || stop "run it as root, such as with" \
		"sudo sh -c 'curl -fsSL https://updates.gophenberg.org/install.sh | sh -s -- --domain example.com'"
	[ "$version" != unstamped ] || stop "this copy of install.sh names no release," \
		"so run it from https://updates.gophenberg.org/install.sh"
	refuse_archive "$@"
	pick_platform
	need curl
	need openssl
	need tar
	check_openssl
	make_work
	name="gophenberg_${version}_linux_$arch.tar.gz"
	say "Downloading Gophenberg $version for linux/$arch"
	fetch "$name"
	fetch "$name.sig"
	check_sum
	check_signature
	say "The archive matches its checksum and a Gophenberg release key signed it"
	hand_over "$@"
}

arguments_given=$#
main "$@"
