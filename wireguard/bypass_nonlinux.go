//go:build !linux
// +build !linux

package wireguard

import "context"

type BypassConfig struct {
	HomeSSIDs     []string `json:"home_ssids"`
	BypassSubnets []string `json:"bypass_subnets"`
}

func StartBypassMonitor(ctx context.Context) {}
func EvaluateBypassRules()                  {}
func FlushBypassRules()                     {}
