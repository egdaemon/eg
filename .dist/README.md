### managing gpg keyrings.

curl -fsSL https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/unstable/xUbuntu_22.04/Release.key | gpg --dearmor | tee derp.key > /dev/null
gpg --no-default-keyring --keyring temp.gpg --import derp.key
gpg --no-default-keyring --keyring temp.gpg --export > derp.gpg


### Debian standards - https://www.debian.org/doc/debian-policy/

policies standards for debian packages. primarily useful for looking up the current value
for the Standards-Version field in the control file.
