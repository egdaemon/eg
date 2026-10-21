#!/bin/bash
# publishes debian packages to an eg deb namespace, served at {namespace}.{account id}.<apt hostname>.
# the repository indexes are generated and signed locally, egmeta verifies the signature against
# the keys registered with the namespace.
#
# usage: publish.sh package.deb...
#
# environment:
#   EG_DEB_TOKEN          (required) bearer token with deb_modify, see `eg deb token`.
#   DEB_NAMESPACE         (required) namespace to publish to.
#   DEB_DISTRO            (required) distribution (codename) to publish into, e.g. resolute.
#   DEB_COMPONENT         component of the distribution, defaults to main.
#   DEB_ARCHITECTURES     space separated architectures of the indexes, defaults to amd64.
#                         architecture all packages are included in every index.
#   DEB_SIGNING_KEY       gpg key used to sign the indexes, defaults to gpg's default key.
#   EG_DEB_API            egmeta api, defaults to https://api.egdaemon.com
#   EG_DEB_APT_HOSTNAME   apt hostname, defaults to apt.egdaemon.com
#   EG_DEB_APT_SCHEME     apt scheme, defaults to https
set -euo pipefail

: "${EG_DEB_TOKEN:?EG_DEB_TOKEN is required}"
: "${DEB_NAMESPACE:?DEB_NAMESPACE is required}"
: "${DEB_DISTRO:?DEB_DISTRO is required}"

API="${EG_DEB_API:-https://api.egdaemon.com}"
APTHOST="${EG_DEB_APT_HOSTNAME:-apt.egdaemon.com}"
APTSCHEME="${EG_DEB_APT_SCHEME:-https}"
COMPONENT="${DEB_COMPONENT:-main}"
ARCHES="${DEB_ARCHITECTURES:-amd64}"
AUTH="Authorization: Bearer ${EG_DEB_TOKEN}"
SIGNWITH=()
if [ -n "${DEB_SIGNING_KEY:-}" ]; then
	SIGNWITH=(--local-user "${DEB_SIGNING_KEY}")
fi

if [ $# -eq 0 ]; then
	echo "no packages to publish" >&2
	exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

# stage the packages into the pool and per architecture trees for indexing.
for deb in "$@"; do
	name="$(dpkg-deb --field "${deb}" Package)"
	arch="$(dpkg-deb --field "${deb}" Architecture)"
	pool="pool/${COMPONENT}/${name:0:1}/${name}"
	mkdir -p "${work}/pool/${pool}"
	cp "${deb}" "${work}/pool/${pool}/"

	targets="${arch}"
	if [ "${arch}" = "all" ]; then
		targets="${ARCHES}"
	elif [[ " ${ARCHES} " != *" ${arch} "* ]]; then
		ARCHES="${ARCHES} ${arch}"
	fi

	for target in ${targets}; do
		mkdir -p "${work}/stage/${target}/${pool}"
		ln -f "${work}/pool/${pool}/$(basename "${deb}")" "${work}/stage/${target}/${pool}/"
	done
done

# upload the packages, content is addressed by digest so repeated uploads are cheap.
while IFS= read -r -d '' deb; do
	digest="sha256:$(sha256sum "${deb}" | cut -d' ' -f1)"
	curl --silent --show-error --fail-with-body -X PUT -H "${AUTH}" --data-binary "@${deb}" "${API}/deb/n/${DEB_NAMESPACE}/${digest}" >/dev/null
	echo "uploaded ${deb#"${work}/pool/"} ${digest}"
done < <(find "${work}/pool" -type f -name '*.deb' -print0)

# merge two Packages indexes, entries from the first take precedence by Filename.
merge() {
	awk 'BEGIN { RS = ""; ORS = "\n\n" }
		{ match($0, /(^|\n)Filename: [^\n]+/); filename = substr($0, RSTART, RLENGTH); sub(/^\n/, "", filename) }
		FNR == NR { seen[filename] = 1; print; next }
		!(filename in seen) { print }' "$1" "$2"
}

for attempt in 1 2 3 4 5; do
	ns="$(curl --silent --show-error --fail-with-body -H "${AUTH}" "${API}/deb/n/${DEB_NAMESPACE}")"
	account="$(jq -r .namespace.account_id <<<"${ns}")"
	repo="${APTSCHEME}://${DEB_NAMESPACE}.${account}.${APTHOST}"
	listing="$(curl --silent --show-error --fail-with-body -H "${AUTH}" "${API}/deb/n/${DEB_NAMESPACE}/files")"
	generation="$(jq -r '.generation // 0' <<<"${listing}")"

	# fetch the current dists so other distributions are carried over unchanged and this
	# distribution's existing packages are retained.
	rm -rf "${work}/repo" && mkdir -p "${work}/repo"
	while IFS= read -r path; do
		mkdir -p "${work}/repo/$(dirname "${path}")"
		curl --silent --show-error --fail -H "${AUTH}" -o "${work}/repo/${path}" "${repo}/${path}"
	done < <(jq -r '.items // [] | .[].path' <<<"${listing}")

	dist="${work}/repo/dists/${DEB_DISTRO}"
	rm -f "${dist}/Release" "${dist}/Release.gpg" "${dist}/InRelease"

	for arch in ${ARCHES}; do
		index="${dist}/${COMPONENT}/binary-${arch}"
		mkdir -p "${index}"
		touch "${work}/existing"
		if [ -f "${index}/Packages" ]; then
			cp "${index}/Packages" "${work}/existing"
		elif [ -f "${index}/Packages.gz" ]; then
			gunzip -c "${index}/Packages.gz" > "${work}/existing"
		fi

		if [ -d "${work}/stage/${arch}" ]; then
			(cd "${work}/stage/${arch}" && apt-ftparchive packages pool) > "${work}/new"
		else
			: > "${work}/new"
		fi

		merge "${work}/new" "${work}/existing" > "${index}/Packages"
		rm -f "${index}/Packages.gz" "${index}/Packages.xz" "${work}/existing" "${work}/new"
		gzip -9 --keep --no-name "${index}/Packages"
	done

	components="$(find "${dist}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | sort | xargs)"
	architectures="$(find "${dist}" -mindepth 2 -maxdepth 2 -type d -name 'binary-*' -printf '%f\n' | sed 's/^binary-//' | sort -u | xargs)"

	apt-ftparchive \
		-o APT::FTPArchive::Release::Origin="${DEB_NAMESPACE}" \
		-o APT::FTPArchive::Release::Label="${DEB_NAMESPACE}" \
		-o APT::FTPArchive::Release::Suite="${DEB_DISTRO}" \
		-o APT::FTPArchive::Release::Codename="${DEB_DISTRO}" \
		-o APT::FTPArchive::Release::Components="${components}" \
		-o APT::FTPArchive::Release::Architectures="${architectures}" \
		-o APT::FTPArchive::Release::Acquire-By-Hash=yes \
		release "${dist}" > "${work}/Release"
	mv "${work}/Release" "${dist}/Release"

	gpg --batch --yes "${SIGNWITH[@]}" --clearsign --output "${dist}/InRelease" "${dist}/Release"
	gpg --batch --yes "${SIGNWITH[@]}" --armor --detach-sign --output "${dist}/Release.gpg" "${dist}/Release"

	form=(-F "generation=${generation}")
	while IFS= read -r -d '' file; do
		rel="${file#"${work}/repo/dists/"}"
		form+=(-F "dists=@${file};filename=${rel}")
	done < <(find "${work}/repo/dists" -type f -print0)

	status="$(curl --silent --show-error --output "${work}/response" --write-out '%{http_code}' -X POST -H "${AUTH}" "${form[@]}" "${API}/deb/n/${DEB_NAMESPACE}/publish")"
	case "${status}" in
	200)
		echo "published ${DEB_NAMESPACE} ${DEB_DISTRO} generation $(jq -r .namespace.generation "${work}/response") to ${repo}"
		exit 0
		;;
	409)
		echo "namespace changed during publish, retrying (${attempt})" >&2
		sleep $((attempt * 2))
		;;
	*)
		echo "publish failed (${status}): $(cat "${work}/response")" >&2
		exit 1
		;;
	esac
done

echo "publish failed, namespace kept changing" >&2
exit 1
