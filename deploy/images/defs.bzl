# SPDX-License-Identifier: MIT
"""hourglass_image: a non-root rules_oci image for one Go binary, for linux/amd64 and linux/arm64.

Per image NAME the macro defines:
  NAME_amd64, NAME_arm64   the single-platform images (built on any host, no Docker)
  NAME_index               both platforms as one OCI index
  NAME                     alias to the image for the host CPU (what oci_load loads)
  load_NAME                oci_load into the local Docker daemon as NAME:dev (tag "manual")
"""

load("@aspect_bazel_lib//lib:tar.bzl", "tar")
load("@rules_go//go:def.bzl", "go_binary")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_image_index", "oci_load")

# arch name -> (rules_go goarch, base image repository, host CPU constraint)
_ARCHS = {
    "amd64": ("amd64", "@distroless_static_linux_amd64", "@platforms//cpu:x86_64"),
    "arm64": ("arm64", "@distroless_static_linux_arm64_v8", "@platforms//cpu:arm64"),
}

NONROOT = "65532"

def hourglass_image(name, lib, binary, ports, description, gotags = []):
    """Defines the image targets for one binary.

    Args:
      name: image name, also the Docker repository (NAME:dev).
      lib: the go_library holding the binary's main package (embed).
      binary: file name of the executable inside the image (/usr/local/bin/BINARY).
      ports: TCP ports the image listens on (the EXPOSE equivalent).
      description: one line for the org.opencontainers.image.description label.
      gotags: Go build tags; ["ui"] embeds the UI.
    """
    images = {}
    for arch, (goarch, base, _) in _ARCHS.items():
        go_binary(
            name = "{}_bin_{}".format(name, arch),
            embed = [lib],
            goos = "linux",
            goarch = goarch,
            pure = "on",  # cgo off: the binaries are static
            gotags = gotags,
            visibility = ["//visibility:private"],
        )
        tar(
            name = "{}_bin_layer_{}".format(name, arch),
            srcs = [":{}_bin_{}".format(name, arch)],
            mtree = [
                "usr uid=0 gid=0 mode=0755 time=0 type=dir",
                "usr/local uid=0 gid=0 mode=0755 time=0 type=dir",
                "usr/local/bin uid=0 gid=0 mode=0755 time=0 type=dir",
                "usr/local/bin/{} uid=0 gid=0 mode=0755 time=0 type=file content=$(location :{}_bin_{})".format(binary, name, arch),
            ],
        )
        oci_image(
            name = "{}_{}".format(name, arch),
            base = base,
            tars = [
                ":{}_bin_layer_{}".format(name, arch),
                ":data_layer",
            ],
            entrypoint = ["/usr/local/bin/" + binary],
            workdir = "/data",
            user = NONROOT + ":" + NONROOT,
            exposed_ports = ["{}/tcp".format(p) for p in ports],
            labels = {
                "org.opencontainers.image.title": name,
                "org.opencontainers.image.description": description,
                "org.opencontainers.image.licenses": "MIT",
                "org.opencontainers.image.base.name": "gcr.io/distroless/static-debian12:nonroot",
                "io.hourglass.ports": ",".join([str(p) for p in ports]),
                "io.hourglass.data-volume": "/data (must be writable by uid/gid " + NONROOT + ")",
            },
            visibility = ["//visibility:public"],
        )
        images[arch] = ":{}_{}".format(name, arch)

    oci_image_index(
        name = name + "_index",
        images = [images["amd64"], images["arm64"]],
        visibility = ["//visibility:public"],
    )
    native.alias(
        name = name,
        actual = select({
            _ARCHS["amd64"][2]: images["amd64"],
            _ARCHS["arm64"][2]: images["arm64"],
        }),
        visibility = ["//visibility:public"],
    )
    oci_load(
        name = "load_" + name,
        image = ":" + name,
        repo_tags = [name + ":dev"],
        tags = ["manual"],
        visibility = ["//visibility:public"],
    )
