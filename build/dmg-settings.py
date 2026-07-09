# dmgbuild settings — see https://dmgbuild.readthedocs.io/
import os

build_dir = defines["build_dir"]

files = [os.path.join(build_dir, "bin", "AndroidFileTransfer.app")]
symlinks = {"Applications": "/Applications"}

format = "UDZO"
filesystem = "HFS+"

icon = os.path.join(build_dir, "appicon.icns")
background = os.path.join(build_dir, "dmg-background.png")

window_rect = ((200, 120), (660, 400))
icon_size = 128
text_size = 13
hide_extensions = ["AndroidFileTransfer.app"]

icon_locations = {
    "AndroidFileTransfer.app": (180, 168),
    "Applications": (480, 168),
}
