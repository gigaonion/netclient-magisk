//go:build !linux

package wireguard

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

func syncPeersRoutes(peers []wgtypes.PeerConfig, replace bool) {}
