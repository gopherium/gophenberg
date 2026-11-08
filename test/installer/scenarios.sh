#!/usr/bin/env bash
set -Eeuo pipefail
trap 'echo "scenarios.sh: line $LINENO ran $BASH_COMMAND, which ended with $?" >&2' ERR

wait_limit=60
late_after_update=20
late_at_boot=30
(( upload_bytes = 40 << 20 ))
update_port=8070
probe_pause=0.2
root_key_iterations=600000
self="$(readlink -f "${BASH_SOURCE[0]}")"
repo="$(dirname "$(dirname "$(dirname "$self")")")"
work="${RUNNER_TEMP:?the scenarios run on a GitHub runner, which sets RUNNER_TEMP}/server-install"
out="$work/out"
front_file="$out/front.txt"
back_file="$out/back.txt"
site="$work/update-site"
cases="$site/install-cases"
install_site="$work/install-site"
install_copy="$work/install-test.sh"
scripts="$work/deploy"
keys="$work/keys"
records="$work/records"
tool="$work/releasetool"
cookies="$work/cookies.txt"
caddy_root="$work/caddy-root.crt"
password_file="$RUNNER_TEMP/admin-password"
admin_email=admin@example.com
admin_name="Maria Perez"
domain=localhost
install_root=/opt/gophenberg
installer_folder=/var/lib/gophenberg-installer
record="$installer_folder/install.json"
server_env=/etc/gophenberg/server.env
command_path=/usr/local/bin/gophenberg
installer_copy=/usr/local/lib/gophenberg/gophenberg
units=/etc/systemd/system
local_root=/var/lib/caddy/.local/share/caddy/pki/authorities/local/root.crt
site_owner=gophenberg:gophenberg
root_owner=root:root
background=()

# fail prints why the scenarios stop on standard error and ends the run with exit 1.
fail() {
	echo "scenarios.sh: $*" >&2
	exit 1
}

# retry runs the command again every second until it succeeds, for the caller to bound with timeout.
retry() {
	until "$@"; do
		sleep 1
	done
}

# wait_until runs the command again every second until it succeeds, for at most wait_limit seconds.
wait_until() {
	timeout "$wait_limit" bash "$self" retry "$@" || fail "waited ${wait_limit}s for $* to succeed"
}

# reset_failed clears the failed state and the start count of the site's unit while systemd has the unit loaded.
reset_failed() {
	if [ "$(systemctl show -p LoadState --value gophenberg.service)" = loaded ]; then
		sudo systemctl reset-failed gophenberg.service
	fi
}

# begin starts the scenario named: it prints the name, notes when the scenario began and runs reset_failed.
begin() {
	scenario="$1"
	scenario_began=$SECONDS
	scenario_started="$EPOCHREALTIME"
	echo "=== $scenario"
	reset_failed
}

# finish prints how long the scenario that begin started took.
finish() {
	echo "=== $scenario took $((SECONDS - scenario_began))s"
}

# logged runs a command with standard input from /dev/null, printing its output and keeping it under the name given.
logged() {
	local name="$1" code=0
	shift
	"$@" < /dev/null 2> "$out/$name.err" | tee "$out/$name.out" || code=$?
	cat "$out/$name.err" >&2
	return "$code"
}

# piped_install pipes the install.sh copy given into sh -s -- under sudo sh -c as the docs do, keeping its output under the name.
piped_install() {
	local name="$1" copy="$2" code=0
	shift 2
	sudo sh -c "cat \"\$0\" | sh -s -- \"\$@\"" "$copy" "$@" < /dev/null 2> "$out/$name.err" | tee "$out/$name.out" || code=$?
	cat "$out/$name.err" >&2
	return "$code"
}

# as_site runs a command of the current build as the site's user in the install root, so it reads the site's .env.
as_site() {
	sudo sh -c 'cd "$1" && shift && exec runuser -u gophenberg -- ./current/gophenberg "$@"' as_site \
		"$install_root" "$@"
}

# status_of prints what the jq filter given picks from update:status -json.
status_of() {
	as_site update:status -json | jq -r "$1"
}

# server_setting prints the value of the line of server.env with the key given.
server_setting() {
	sudo sed -n "s/^$1=//p" "$server_env"
}

# set_server_setting writes the value given in place of the value of the line of server.env with the key given.
set_server_setting() {
	sudo sed -i "s/^$1=.*/$1=$2/" "$server_env"
	[ "$(server_setting "$1")" = "$2" ] || fail "$server_env holds no line $1"
}

# seconds_of prints a duration written in whole seconds, such as 10s, as a number.
seconds_of() {
	[[ $1 =~ ^([0-9]+)s$ ]] || fail "the duration $1 is not written in whole seconds"
	echo "${BASH_REMATCH[1]}"
}

# add_site_line adds the line given to the site's .env as the site's user.
add_site_line() {
	printf '%s\n' "$1" | sudo -u gophenberg tee -a "$install_root/.env" > /dev/null
}

# set_site_setting writes the value given in place of the value of the site's .env line with the key given.
set_site_setting() {
	sudo -u gophenberg sed -i "s/^$1=.*/$1=$2/" "$install_root/.env"
	holds "$install_root/.env" "$1=$2"
}

# drop_site_setting removes the line of the site's .env with the key given, as the site's user.
drop_site_setting() {
	sudo -u gophenberg sed -i "/^$1=/d" "$install_root/.env"
}

# holds stops unless the file given holds the whole line given, read as root.
holds() {
	sudo grep -qx -- "$2" "$1" || fail "$1 holds no line $2"
}

# root_copy_of writes into the runner's file named first what the command after it prints when root runs it.
root_copy_of() {
	local file="$1"
	shift
	sudo "$@" | tee "$file" > /dev/null
}

# root_copy copies the file named first, which only root reads, into the runner's file named second.
root_copy() {
	root_copy_of "$2" cat "$1"
}

# sum_of prints the SHA-256 of a file only root reads.
sum_of() {
	sudo cat "$1" | sha256sum
}

# fetch asks the site through Caddy over HTTPS, trusting Caddy's local root and keeping the sign-in cookie.
fetch() {
	curl -s --max-time 30 --cacert "$caddy_root" -b "$cookies" -c "$cookies" "$@"
}

# status_code prints the status the site answers the request given with through fetch.
status_code() {
	fetch -o /dev/null -w '%{http_code}' "$@" || true
}

# sign_in signs the first administrator in through POST /api/auth/login, keeping the sign-in cookie.
sign_in() {
	local body code
	body="$(jq -nc --arg email "$admin_email" --rawfile password "$password_file" \
		'{email: $email, password: ($password | rtrimstr("\n"))}')"
	rm -f "$cookies"
	code="$(fetch -o "$work/sign-in.json" -w '%{http_code}' -H 'Content-Type: application/json' --data "$body" \
		"https://$domain/api/auth/login" || true)"
	[ "$code" = 200 ] || fail "the first administrator could not sign in, the site answered $code"
}

# served_version prints the version the site answers /api/version with, signed in as the first administrator.
served_version() {
	fetch -f "https://$domain/api/version" | jq -r .version
}

# serves reports whether the site answers through Caddy as the version given.
serves() {
	[ "$(served_version 2> /dev/null)" = "$1" ]
}

# settled reports whether the last update stands at the step given while the site answers as the version given.
settled() {
	[ "$(status_of .update.step 2> /dev/null)" = "$1" ] && serves "$2"
}

# checked reports whether a check from the admin answers and offers the version given.
checked() {
	[ "$(fetch -f --data '' "https://$domain/api/updates/check" | jq -r .offer.version)" = "$1" ]
}

# journal prints what systemd and the unit given logged since the scenario began, one message a line.
journal() {
	sudo journalctl --no-pager -o cat -u "$1" --since "@$scenario_started"
}

# rollback_runs prints how many times the rollback unit started since the scenario began.
rollback_runs() {
	journal gophenberg-rollback.service > "$work/journal.txt"
	grep -cE '^Starting (gophenberg-rollback\.service|Roll Gophenberg back)' "$work/journal.txt" || true
}

# rolled_back stops unless the rollback unit started as many times as given since the scenario began.
rolled_back() {
	local runs
	runs="$(rollback_runs)"
	[ "$runs" = "$1" ] || fail "the rollback unit started $runs times in the scenario $scenario, want $1"
}

# starts_before_limit prints how many times the journal file shows the site's unit started before its start limit hit.
starts_before_limit() {
	awk '/Start request repeated too quickly/ { exit }
		/^Started (gophenberg\.service|Gophenberg\.$)/ { starts++ }
		END { print starts + 0 }' "$1"
}

# owned_as stops unless stat shows the path given with the owner, the group and the mode given.
owned_as() {
	local held
	held="$(sudo stat -c '%U:%G %a' "$1")"
	[ "$held" = "$2 $3" ] || fail "$1 is $held, want $2 $3"
}

# check_owners stops unless every path the install wrote has the owner, the group and the mode the install promises.
check_owners() {
	local path
	owned_as "$install_root" "$site_owner" 750
	owned_as "$install_root/versions" "$site_owner" 755
	owned_as "$install_root/downloads" "$site_owner" 755
	owned_as "$install_root/state" "$site_owner" 700
	owned_as "$install_root/backups" "$site_owner" 700
	owned_as "$install_root/media" "$site_owner" 750
	owned_as "$install_root/themes" "$site_owner" 750
	owned_as "$install_root/.env" "$site_owner" 600
	owned_as "$install_root/current" "$site_owner" 777
	owned_as "$install_root/versions/$a/gophenberg" "$site_owner" 755
	owned_as /etc/gophenberg "$root_owner" 755
	owned_as "$server_env" "$root_owner" 600
	owned_as "$installer_folder" "$root_owner" 700
	owned_as "$record" "$root_owner" 600
	owned_as "$installer_copy" "$root_owner" 755
	owned_as "$command_path" "$root_owner" 755
	for path in /usr/local/share/gophenberg/maintenance/index.html "$units/gophenberg.service" \
		"$units/gophenberg-rollback.service" "$units/caddy.service.d/gophenberg.conf" /etc/caddy/Caddyfile \
		/etc/caddy/gophenberg.caddy /etc/apt/keyrings/caddy.asc /etc/apt/sources.list.d/caddy.sources \
		/etc/apt/preferences.d/gophenberg-caddy; do
		owned_as "$path" "$root_owner" 644
	done
	if [ -e /usr/bin/unattended-upgrade ]; then
		owned_as /etc/apt/apt.conf.d/52gophenberg-caddy "$root_owner" 644
	fi
}

# caddy_admin_closed stops when the site's user reaches Caddy's admin endpoint on its port or on its socket.
caddy_admin_closed() {
	sudo test -S /run/caddy/admin.sock || fail "Caddy keeps no admin socket at /run/caddy/admin.sock"
	if sudo -u gophenberg curl -s -o /dev/null --max-time 5 http://127.0.0.1:2019/config/; then
		fail "the site's user reaches Caddy's admin endpoint on 127.0.0.1:2019"
	fi
	if sudo -u gophenberg curl -s -o /dev/null --max-time 5 --unix-socket /run/caddy/admin.sock \
		http://localhost/config/; then
		fail "the site's user reaches Caddy's admin endpoint on /run/caddy/admin.sock"
	fi
}

# no_https_warning stops when the server:install run kept under the name given warned that HTTPS did not answer.
no_https_warning() {
	if grep -q 'did not answer 200 within' "$out/$1.err"; then
		fail "server:install warned that https://$domain/admin/ did not answer"
	fi
}

# gone stops when the path given is still there, as a file, a folder or a link.
gone() {
	if sudo test -e "$1" || sudo test -L "$1"; then
		fail "server:uninstall left $1"
	fi
}

# kept stops unless the path given is still there.
kept() {
	sudo test -e "$1" || fail "server:uninstall removed $1"
}

# not_made stops when the path given is there after the copy of install.sh named stopped, as a file, a folder or a link.
not_made() {
	if sudo test -e "$2" || sudo test -L "$2"; then
		fail "the copy $1 of install.sh stopped and still made $2"
	fi
}

# no_work_folder stops when install.sh left a download folder in the installer folder.
no_work_folder() {
	local left
	left="$(sudo find "$installer_folder" -mindepth 1 -maxdepth 1 -type d -name 'download.*')"
	[ -z "$left" ] || fail "install.sh left its download folder $left"
}

# has_line stops unless the file given, which the runner reads, holds the whole line given.
has_line() {
	grep -qxF -- "$2" "$1" || fail "$1 holds no line $2"
}

# checksum_of prints the SHA-256 of a file the runner reads, in hex alone.
checksum_of() {
	local line
	line="$(sha256sum < "$1")"
	echo "${line%% *}"
}

# probe asks the address every probe pause and adds when it asked, the status and how long the answer took to the file.
probe() {
	local file="$1" address="$2" asked answer
	shift 2
	while :; do
		asked="$EPOCHREALTIME"
		answer="$(curl -s -o /dev/null --max-time 30 -w '%{http_code} %{time_total}' "$@" "$address" || true)"
		printf '%s %s\n' "$asked" "$answer" >> "$file"
		sleep "$probe_pause"
	done
}

# start_probes asks the home page through Caddy and the site's own address every probe pause in the background.
start_probes() {
	: > "$front_file"
	: > "$back_file"
	timeout "$wait_limit" bash "$self" probe "$front_file" "https://$domain/" --cacert "$caddy_root" \
		> /dev/null 2>&1 &
	front_probe=$!
	background+=("$front_probe")
	timeout "$wait_limit" bash "$self" probe "$back_file" "http://$(server_setting GOPHENBERG_ADDR)/" \
		> /dev/null 2>&1 &
	back_probe=$!
	background+=("$back_probe")
	sleep 1
}

# stop_probes stops both probes once they asked the new build too, and waits for them to end.
stop_probes() {
	sleep 1
	kill "$front_probe" "$back_probe" 2> /dev/null || true
	wait "$front_probe" "$back_probe" 2> /dev/null || true
}

# gap_of prints the longest time in seconds the site's own address went without an answer, from the file of a probe.
gap_of() {
	awk '$2 == "000" { if (last) down = 1; next }
		{ if (down && $1 - last > gap) gap = $1 - last; last = $1; down = 0 }
		END { printf "%.3f\n", gap }' "$1"
}

# longest_of prints the longest time in seconds one request took, from the file of a probe.
longest_of() {
	awk '$3 > longest { longest = $3 } END { printf "%.3f\n", longest }' "$1"
}

# below reports whether the first number is smaller than the second.
below() {
	awk -v first="$1" -v second="$2" 'BEGIN { exit !(first < second) }'
}

# answered_only stops when a request of the probe file got a status the pattern does not name, such as 200|503.
answered_only() {
	local others
	others="$(awk -v allowed="^($2)\$" '$2 !~ allowed' "$1")"
	[ -z "$others" ] || fail "requests through Caddy got other answers than $2: $others"
}

# judge_held stops unless, under the retry duration, Caddy answered 200 alone and held a request past the held floor.
judge_held() {
	local gap="$1" longest="$2" delay="$3" retry="$4" floor
	if ! below "$gap" "$retry"; then
		echo "the gap passed the retry duration of ${retry}s, so the maintenance page may answer"
		answered_only "$front_file" '200|503'
		return
	fi
	answered_only "$front_file" 200
	floor="$(held_floor "$delay")"
	below "$floor" "$longest" ||
		fail "no request through Caddy was held past ${floor}s, the restart delay of ${delay}s less one probe pause"
}

# held_floor prints the restart delay given less one probe pause, the least a request asked during the gap is held.
held_floor() {
	awk -v delay="$1" -v pause="$probe_pause" 'BEGIN { printf "%.3f\n", delay - pause }'
}

# host_facts prints what the runner image brings that the install depends on, and what already listens.
host_facts() {
	systemctl --version
	ps -p 1 -o comm=
	openssl version
	apt-cache policy postgresql caddy
	free -m
	df -h
	sudo ss -ltnup
	stat -c '%U:%G %a %n' / /opt /usr/local/bin
}

# purge_postgresql purges the image's PostgreSQL packages, their folders and the PostgreSQL project's apt source.
purge_postgresql() {
	local -a packages
	local source
	mapfile -t packages < <(dpkg-query -W -f '${db:Status-Abbrev} ${binary:Package}\n' 'postgresql*' 2> /dev/null |
		awk '$1 !~ /^u/ { print $2 }')
	if [ "${#packages[@]}" -gt 0 ]; then
		sudo env DEBIAN_FRONTEND=noninteractive apt-get purge -y -qq "${packages[@]}"
	fi
	sudo rm -rf /etc/postgresql /var/lib/postgresql /usr/lib/postgresql
	for source in /etc/apt/sources.list.d/*; do
		if grep -qs apt.postgresql.org "$source"; then
			sudo rm -f "$source"
		fi
	done
}

# close_opt gives /opt back to root alone at mode 0755, as a fresh Ubuntu 24.04 server has it.
close_opt() {
	sudo chown root:root /opt
	sudo chmod 0755 /opt
}

# read_versions sets the checkout's version and the versions of A, B, D and the stub C1, the four patches after it.
read_versions() {
	local major minor patch
	IFS=. read -r major minor patch < "$repo/internal/version/VERSION"
	base="$major.$minor.$patch"
	a="$major.$minor.$((patch + 1))"
	b="$major.$minor.$((patch + 2))"
	d="$major.$minor.$((patch + 3))"
	c1="$major.$minor.$((patch + 4))"
	echo "the checkout is $base, the archives are A $a, B $b and D $d, and the stub is C1 $c1"
}

# build_releasetool builds releasetool from the standard library alone, as the release workflow does.
build_releasetool() {
	GOMODCACHE="$work/empty-module-cache" GOPROXY=off GOFLAGS=-mod=readonly GOWORK=off \
		go -C "$repo" build -o "$tool" ./cmd/releasetool
}

# archive_name prints the name of the linux/amd64 archive of the version given.
archive_name() {
	echo "gophenberg_$1_linux_amd64.tar.gz"
}

# archive prints where the linux/amd64 archive of the version given is built.
archive() {
	echo "$work/dist-$1/$(archive_name "$1")"
}

# build_archives builds the official linux/amd64 archives A, B and D from the checkout, each under its own version.
build_archives() {
	local version
	for version in "$a" "$b" "$d"; do
		printf '%s\n' "$version" > "$repo/internal/version/VERSION"
		make -C "$repo" release-archives RELEASE_TARGETS=linux/amd64 RELEASE_STAMP=official \
			RELEASE_DIR="$work/dist-$version"
	done
	printf '%s\n' "$base" > "$repo/internal/version/VERSION"
}

# pack_stub packs C1, whose gophenberg answers about -json with B's release.json as C1 and exits 1 on serve.
pack_stub() {
	local stub="$work/stub"
	mkdir -p "$stub" "$work/dist-$c1"
	tar -xzOf "$(archive "$b")" release.json |
		jq --arg version "$c1" '.version = $version | .stamp = "official"' > "$stub/release.json"
	printf '%s\n' '#!/bin/sh' "if [ \"\${1-}\" = about ]; then" "	exec cat \"\$(dirname \"\$0\")/release.json\"" \
		'fi' 'exit 1' > "$stub/gophenberg"
	chmod 0755 "$stub/gophenberg"
	cp "$repo/LICENSE" "$stub/LICENSE"
	tar -C "$stub" --owner=0 --group=0 --numeric-owner -czf "$(archive "$c1")" gophenberg release.json LICENSE
}

# multikey prints the Multikey of the throwaway key of the role given.
multikey() {
	cat "$keys/$1.multikey"
}

# on_site runs the releasetool command given over the local update site, naming its keys and its throwaway root.
on_site() {
	local command="$1"
	shift
	"$tool" "$command" -site "$site" -release "$(multikey release)" -standby "$(multikey standby)" \
		-timestamp "$(multikey timestamp)" -root "$(multikey root)" "$@"
}

# publish signs the archive of the version given with the release key and writes its record for the local update site.
publish() {
	local folder="$site/releases/v$1" name
	name="$(archive_name "$1")"
	mkdir -p "$folder"
	cp "$(archive "$1")" "$folder/$name"
	"$tool" sign -key "$keys/release.pem" "$folder/$name"
	"$tool" record -version "$1" -download-url "http://127.0.0.1:$update_port/releases/v$1" "$folder/$name" \
		> "$records/$1.json"
	tar -xzOf "$folder/$name" release.json > "$records/$1.release.json"
}

# offer adds the version given to the stable index of the local update site, then signs it and a new timestamp file.
offer() {
	local index="$site/v1/core/stable.json" note="$site/v1/timestamp.json"
	on_site index -channel stable -record "$records/$1.json" -release-json "$records/$1.release.json" -valid 24h \
		> "$work/stable.json"
	mv "$work/stable.json" "$index"
	rm -f "$index.sig"
	"$tool" sign -key "$keys/release.pem" "$index"
	on_site timestamp -newest "stable=$1" -valid 24h > "$work/timestamp.json"
	mv "$work/timestamp.json" "$note"
	rm -f "$note.sig"
	"$tool" sign -key "$keys/timestamp.pem" "$note"
}

# make_update_site makes the throwaway keys, the signed key list and every record, then offers A and B.
make_update_site() {
	local role version
	openssl rand -hex 32 > "$keys/root-passphrase"
	"$tool" keygen -curve p256 -out "$keys/root.pem" -passphrase-file "$keys/root-passphrase" \
		-iterations "$root_key_iterations" | awk '$1 == "multikey" { print $2 }' > "$keys/root.multikey"
	for role in release standby timestamp; do
		"$tool" keygen -curve ed25519 -out "$keys/$role.pem" | awk '$1 == "multikey" { print $2 }' \
			> "$keys/$role.multikey"
	done
	"$tool" keys -release "$(multikey release)" -standby "$(multikey standby)" -timestamp "$(multikey timestamp)" \
		> "$site/v1/keys.json"
	"$tool" sign-keys -key "$keys/root.pem" -passphrase-file "$keys/root-passphrase" "$site/v1/keys.json"
	for version in "$a" "$b" "$d" "$c1"; do
		publish "$version"
	done
	offer "$a"
	offer "$b"
}

# serve_update_site serves the local update site on the loopback address in the background.
serve_update_site() {
	python3 -m http.server --bind 127.0.0.1 --directory "$site" "$update_port" > "$out/update-site.log" 2>&1 &
	background+=("$!")
	wait_until curl -fsS -o /dev/null "http://127.0.0.1:$update_port/v1/keys.json"
}

# stop_background stops every process the run started in the background.
stop_background() {
	if [ "${#background[@]}" -gt 0 ]; then
		kill "${background[@]}" 2> /dev/null || true
	fi
}

# script_record signs two copies of A's archive named for the version given, amd64 and arm64, then their signed record.
script_record() {
	local folder="$work/script-records/$1"
	mkdir -p "$folder"
	cp "$(archive "$a")" "$folder/gophenberg_$1_linux_amd64.tar.gz"
	cp "$(archive "$a")" "$folder/gophenberg_$1_linux_arm64.tar.gz"
	"$tool" sign -key "$keys/release.pem" "$folder"/*.tar.gz
	"$tool" record -version "$1" -download-url "http://127.0.0.1:$update_port/releases/v$1" "$folder"/*.tar.gz \
		> "$records/$1.script.json"
	"$tool" sign -key "$keys/release.pem" "$records/$1.script.json"
}

# public_body prints the one base64 line of the PEM that holds the public half of the throwaway release key.
public_body() {
	openssl pkey -in "$keys/release.pem" -pubout | sed -n 2p
}

# trusting_scripts copies publish-install.sh and install.sh into the work folder, install.sh naming the throwaway key.
trusting_scripts() {
	local key
	key="$(public_body)"
	mkdir -p "$scripts"
	cp "$repo/deploy/publish-install.sh" "$scripts/publish-install.sh"
	sed -e "s|^release_key=.*|release_key=$key|" -e "s|^standby_key=.*|standby_key=$key|" "$repo/deploy/install.sh" \
		> "$scripts/install.sh"
	has_line "$scripts/install.sh" "release_key=$key"
	has_line "$scripts/install.sh" "standby_key=$key"
}

# publish_install_script runs publish-install.sh with the version's record and the throwaway key over the install site.
publish_install_script() {
	logged "$1" sh "$scripts/publish-install.sh" "$tool" "$records/$2.script.json" "$install_site" \
		"$keys/release.pem" "$(multikey release)" || fail "publish-install.sh did not end with exit 0 for $2"
}

# printed stops unless the command kept under the name given printed the one line given on standard output.
printed() {
	local said
	said="$(cat "$out/$1.out")"
	[ "$said" = "$2" ] || fail "$1 printed $said, want $2"
}

# test_copy copies install.sh to take http, kept to loopback by the download address and http.server's 127.0.0.1 bind.
test_copy() {
	local copy="$2" key line
	local -a lines
	key="$(public_body)"
	lines=("downloads=$1" "protocols='=http,https'" "release_key=$key" "standby_key=$key")
	if [ "$#" -gt 2 ]; then
		lines+=("sha256_amd64=$3")
	fi
	cp "$install_site/install.sh" "$copy"
	for line in "${lines[@]}"; do
		sed -i "s|^${line%%=*}=.*|$line|" "$copy"
		has_line "$copy" "$line"
	done
	has_line "$copy" "version=$a"
}

# serve_case serves the archive and the .sig given under A's names in the case folder named on the local update site.
serve_case() {
	local folder="$cases/$1/v$a" name
	name="$(archive_name "$a")"
	mkdir -p "$folder"
	cp "$2" "$folder/$name"
	cp "$3" "$folder/$name.sig"
}

# stop_case runs the install.sh copy given as root, which must end with exit 1 on the line given and make nothing.
stop_case() {
	local copy="$1" line="$2" name code=0
	shift 2
	name="$(basename "$copy" .sh)"
	piped_install "$name" "$copy" "$@" || code=$?
	[ "$code" = 1 ] || fail "the copy $name of install.sh ended with $code, want 1"
	[ "$(tail -n 1 "$out/$name.err")" = "install.sh: $line" ] ||
		fail "the copy $name of install.sh did not stop on the line install.sh: $line"
	not_made "$name" "$install_root"
	not_made "$name" "$installer_copy"
	no_work_folder
}

# scenario_publish_install publishes install.sh for the checkout's version, then A over it, then the older one again.
scenario_publish_install() {
	local sum signed published line
	begin "publishing install.sh"
	trusting_scripts
	script_record "$base"
	script_record "$a"
	publish_install_script publish-base "$base"
	printed publish-base "install.sh now names Gophenberg $base"
	has_line "$install_site/install.sh" "version=$base"
	signed="$(checksum_of "$install_site/install.sh.sig")"
	publish_install_script publish-a "$a"
	printed publish-a "install.sh now names Gophenberg $a"
	sum="$(checksum_of "$(archive "$a")")"
	for line in "version=$a" "sha256_amd64=$sum" "sha256_arm64=$sum"; do
		has_line "$install_site/install.sh" "$line"
	done
	[ "$(checksum_of "$install_site/install.sh.sig")" != "$signed" ] ||
		fail "publish-install.sh kept the .sig of install.sh for $base when it published $a"
	"$tool" verify -public "$(multikey release)" "$install_site/install.sh" ||
		fail "install.sh for $a does not verify under the throwaway release key"
	published="$(sha256sum "$install_site/install.sh" "$install_site/install.sh.sig")"
	publish_install_script publish-base-again "$base"
	printed publish-base-again \
		"the update site's install.sh names $a, which is not older than $base, so it stays as it is"
	[ "$(sha256sum "$install_site/install.sh" "$install_site/install.sh.sig")" = "$published" ] ||
		fail "publish-install.sh changed install.sh or its .sig for the older $base"
	finish
}

# scenario_install_script_stops runs install.sh on another archive, a tampered one and with -archive, and each stops.
scenario_install_script_stops() {
	local name tampered
	name="$(archive_name "$a")"
	tampered="$cases/tampered/v$a/$name"
	begin "install.sh stops"
	serve_case another-checksum "$(archive "$b")" "$site/releases/v$b/$(archive_name "$b").sig"
	test_copy "http://127.0.0.1:$update_port/install-cases/another-checksum" "$work/another-checksum.sh"
	stop_case "$work/another-checksum.sh" "the archive is not the one this install.sh names, so nothing was installed" \
		--domain "$domain"
	serve_case tampered "$(archive "$a")" "$site/releases/v$a/$name.sig"
	printf 'x' >> "$tampered"
	test_copy "http://127.0.0.1:$update_port/install-cases/tampered" "$work/tampered.sh" "$(checksum_of "$tampered")"
	stop_case "$work/tampered.sh" \
		"the archive's signature matches neither Gophenberg release key, so nothing was installed" --domain "$domain"
	stop_case "$install_copy" "install.sh passes the archive it checked itself, so leave out -archive" \
		--domain "$domain" --archive "$installer_folder/a.tar.gz"
	finish
}

# scenario_install installs A through the test copy of install.sh with every flag of the first administrator.
scenario_install() {
	begin "install"
	(umask 077 && openssl rand -base64 18 > "$password_file")
	piped_install install "$install_copy" --domain "$domain" --admin-email "$admin_email" --admin-name "$admin_name" \
		--admin-password-file "$password_file" || fail "the first install through install.sh did not end with exit 0"
	no_work_folder
	no_https_warning install
	grep -q "Gophenberg $a runs at https://$domain" "$out/install.out" ||
		fail "the summary names no Gophenberg $a running at https://$domain"
	root_copy "$local_root" "$caddy_root"
	[ "$(status_code "https://$domain/admin/")" = 200 ] || fail "https://$domain/admin/ does not answer 200"
	sign_in
	check_install
	finish
}

# check_install stops unless .env, the units, Caddy's admin endpoint, the command and every owner are as promised.
check_install() {
	holds "$install_root/.env" GOPHENBERG_SUPERUSER_POLICY=refuse
	holds "$units/gophenberg.service" RestartMode=direct
	holds "$units/gophenberg.service" StartLimitBurst=10
	holds "$units/gophenberg-rollback.service" StartLimitIntervalSec=0
	caddy_admin_closed
	shellcheck -s sh "$command_path"
	check_owners
	sum_of /etc/caddy/Caddyfile.before-gophenberg > "$work/package-caddyfile.sum"
}

# scenario_update_site points the site at the local update site and its throwaway root, then restarts it.
scenario_update_site() {
	begin "the local update site"
	add_site_line "GOPHENBERG_UPDATE_URL=http://127.0.0.1:$update_port"
	add_site_line "GOPHENBERG_TRUSTED_KEYS=$(multikey root)"
	add_site_line GOPHENBERG_UPDATES_SCREEN_CHECK_AGE=1s
	sudo systemctl restart gophenberg.service
	wait_until serves "$a"
	finish
}

# scenario_update updates A to B from the admin while two probes measure the gap and the requests Caddy held.
scenario_update() {
	local delay retry code gap longest
	begin "an update from the admin"
	delay="$(seconds_of "$(server_setting GOPHENBERG_SERVICE_RESTART_DELAY)")"
	retry="$(seconds_of "$(server_setting GOPHENBERG_PROXY_RETRY_DURATION)")"
	[ "$(status_code "https://$domain/")" = 200 ] || fail "https://$domain/ does not answer 200 before the update"
	wait_until checked "$b"
	start_probes
	code="$(fetch -o "$work/apply.json" -w '%{http_code}' -H 'Content-Type: application/json' \
		--data "{\"version\":\"$b\"}" "https://$domain/api/updates/apply" || true)"
	[ "$code" = 202 ] || fail "the start of the update to $b from the admin answered $code"
	wait_until settled committed "$b"
	stop_probes
	gap="$(gap_of "$back_file")"
	longest="$(longest_of "$front_file")"
	echo "the update gap was ${gap}s and the longest request Caddy held took ${longest}s"
	judge_held "$gap" "$longest" "$delay" "$retry"
	[ "$(status_of '.backups | length')" -ge 1 ] || fail "the update to $b kept no backup in $install_root/backups"
	rolled_back 0
	finish
}

# scenario_upload uploads a 40 MB big.mp4 of random bytes through Caddy and keeps where the site stored it.
scenario_upload() {
	local code
	begin "a 40 MB upload"
	head -c "$upload_bytes" /dev/urandom > "$work/big.mp4"
	code="$(fetch -o "$work/upload.json" -w '%{http_code}' -F "file=@$work/big.mp4" "https://$domain/api/media" ||
		true)"
	[ "$code" = 201 ] || fail "the upload of big.mp4 answered $code, want 201"
	jq -r .file "$work/upload.json" > "$work/big.path"
	finish
}

# scenario_second_run changes two timings of server.env and two lines of .env, then runs sudo gophenberg server:install.
scenario_second_run() {
	begin "a second run"
	[ "$(sudo sh -c 'command -v gophenberg')" = "$command_path" ] ||
		fail "sudo finds no gophenberg command at $command_path"
	set_server_setting GOPHENBERG_SERVICE_RESTART_DELAY 2s
	set_server_setting GOPHENBERG_SERVICE_START_LIMIT_INTERVAL 60s
	add_site_line GOPHENBERG_HTTP_IDLE_TIMEOUT=2m
	set_site_setting GOPHENBERG_DATABASE_WAIT 10s
	logged second-run sudo gophenberg server:install || fail "the second server:install did not end with exit 0"
	no_https_warning second-run
	holds "$units/gophenberg.service" RestartSec=2s
	holds "$units/gophenberg.service" StartLimitIntervalSec=60
	holds "$install_root/.env" GOPHENBERG_HTTP_IDLE_TIMEOUT=2m
	holds "$install_root/.env" GOPHENBERG_DATABASE_WAIT=10s
	wait_until serves "$b"
	finish
}

# scenario_failed_check offers D, which fails its start check under a 1ms verify timeout, and the rollback unit runs.
scenario_failed_check() {
	begin "a failed start check"
	add_site_line GOPHENBERG_UPDATE_VERIFY_TIMEOUT=1ms
	offer "$d"
	logged apply-d sudo gophenberg update:apply -to "$d" ||
		fail "sudo gophenberg update:apply -to $d did not end with exit 0"
	wait_until settled rolled-back "$b"
	journal gophenberg.service > "$work/journal.txt"
	grep -q 'status=76' "$work/journal.txt" || fail "the journal shows no exit status 76 of Gophenberg $d"
	rolled_back 1
	drop_site_setting GOPHENBERG_UPDATE_VERIFY_TIMEOUT
	finish
}

# scenario_late_database_after_update switches to D again, then starts it while PostgreSQL stays down for a while.
scenario_late_database_after_update() {
	local code=0
	begin "a late database after an update"
	logged apply-d-retry as_site update:apply -to "$d" -retry || code=$?
	[ "$code" = 75 ] || fail "update:apply -to $d -retry ended with $code, want 75"
	sudo systemctl stop postgresql.service
	sudo systemctl reset-failed gophenberg.service
	sudo systemctl restart gophenberg.service
	sleep "$late_after_update"
	sudo systemctl start postgresql.service
	wait_until settled committed "$d"
	rolled_back 0
	finish
}

# scenario_late_database_at_boot restarts the site while PostgreSQL stays down for a while, with no update pending.
scenario_late_database_at_boot() {
	begin "a late database at boot"
	sudo systemctl stop postgresql.service
	sudo systemctl restart gophenberg.service
	sleep "$late_at_boot"
	sudo systemctl start postgresql.service
	wait_until settled committed "$d"
	rolled_back 0
	finish
}

# scenario_start_limit offers C1, whose serve fails at once, so the start limit hits and the rollback unit runs.
scenario_start_limit() {
	local starts
	begin "the start limit"
	offer "$c1"
	logged apply-c1 sudo gophenberg update:apply -to "$c1" ||
		fail "sudo gophenberg update:apply -to $c1 did not end with exit 0"
	wait_until settled rolled-back "$d"
	journal gophenberg.service > "$work/journal.txt"
	grep -q 'Start request repeated too quickly' "$work/journal.txt" ||
		fail "the journal shows no start limit of Gophenberg $c1"
	starts="$(starts_before_limit "$work/journal.txt")"
	echo "Gophenberg $c1 started $starts times before the start limit hit"
	[ "$starts" = "$(server_setting GOPHENBERG_SERVICE_START_LIMIT_BURST)" ] ||
		fail "Gophenberg $c1 started $starts times, not the start limit burst"
	rolled_back 1
	if logged apply-c1-again sudo gophenberg update:apply -to "$c1"; then
		fail "a second update:apply -to $c1 went through without -retry"
	fi
	grep -q -- -retry "$out/apply-c1-again.out" "$out/apply-c1-again.err" ||
		fail "the second update:apply -to $c1 does not name -retry"
	finish
}

# scenario_uninstall runs sudo gophenberg server:uninstall -yes and checks what it removed and what it kept.
scenario_uninstall() {
	local path
	begin "server:uninstall"
	logged uninstall sudo gophenberg server:uninstall -yes || fail "server:uninstall -yes did not end with exit 0"
	for path in "$units/gophenberg.service" "$units/gophenberg-rollback.service" \
		"$units/caddy.service.d/gophenberg.conf" /etc/caddy/gophenberg.caddy /etc/caddy/Caddyfile.before-gophenberg \
		/usr/local/share/gophenberg/maintenance /usr/local/share/gophenberg "$command_path" "$installer_copy" \
		/usr/local/lib/gophenberg "$install_root/versions" "$install_root/downloads" "$install_root/current" \
		"$install_root/previous" "$install_root/state/update.json"; do
		gone "$path"
	done
	[ "$(sum_of /etc/caddy/Caddyfile)" = "$(cat "$work/package-caddyfile.sum")" ] ||
		fail "server:uninstall did not put the Caddy package's Caddyfile back"
	for path in "$install_root/.env" "$install_root/media" "$server_env" "$record"; do
		kept "$path"
	done
	[ -n "$(sudo ls -A "$install_root/media")" ] || fail "server:uninstall emptied $install_root/media"
	finish
}

# scenario_reinstall installs A again through install.sh with no admin flag, then signs in and fetches the kept big.mp4.
scenario_reinstall() {
	local fetched
	begin "a reinstall"
	piped_install reinstall "$install_copy" --domain "$domain" ||
		fail "the reinstall through install.sh did not end with exit 0"
	no_work_folder
	no_https_warning reinstall
	sign_in
	wait_until serves "$a"
	fetched="$(fetch -o /dev/null -w '%{http_code} %{size_download}' "https://$domain/media/$(cat "$work/big.path")" ||
		true)"
	[ "$fetched" = "200 $upload_bytes" ] || fail "the kept big.mp4 answered $fetched, want 200 $upload_bytes"
	finish
}

# run builds the archives and the local update site, then runs every scenario in order.
run() {
	mkdir -p "$out" "$site/v1/core" "$keys" "$records" "$install_site"
	trap stop_background EXIT
	host_facts
	purge_postgresql
	close_opt
	read_versions
	build_releasetool
	build_archives
	pack_stub
	make_update_site
	serve_update_site
	echo "=== the archives and the local update site took ${SECONDS}s"
	cd / || fail "cannot change to /"
	scenario_publish_install
	test_copy "http://127.0.0.1:$update_port/releases" "$install_copy"
	scenario_install_script_stops
	scenario_install
	scenario_update_site
	scenario_update
	scenario_upload
	scenario_second_run
	scenario_failed_check
	scenario_late_database_after_update
	scenario_late_database_at_boot
	scenario_start_limit
	scenario_uninstall
	scenario_reinstall
	echo "=== every scenario passed, and the run took ${SECONDS}s"
}

# gather_logs copies the journals, the update state, the install record and the command outputs, never .env.
gather_logs() {
	local folder="$1" unit
	mkdir -p "$folder"
	for unit in gophenberg gophenberg-rollback caddy 'postgresql*'; do
		root_copy_of "$folder/${unit%\*}.log" journalctl --no-pager -u "$unit" || true
	done
	root_copy "$install_root/state/update.json" "$folder/update.json" || true
	root_copy "$record" "$folder/install.json" || true
	if [ -d "$out" ]; then
		cp -R "$out" "$folder/out"
	fi
}

# main runs what the first argument names: run, logs <folder>, or the retry and probe the waits start.
main() {
	case "${1-}" in
	run)
		run
		;;
	logs)
		gather_logs "${2:?scenarios.sh logs needs the folder to gather the logs into}"
		;;
	retry)
		trap - ERR
		shift
		retry "$@"
		;;
	probe)
		trap - ERR
		shift
		probe "$@"
		;;
	*)
		fail "usage: scenarios.sh run, or scenarios.sh logs <folder>"
		;;
	esac
}

main "$@"
