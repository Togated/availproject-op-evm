package config

import (
	"errors"
	"net"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/server/config"
	"github.com/0xPolygon/polygon-edge/network/common"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/multiformats/go-multiaddr"
)

func ParseGenesisConfig(cfg *config.Config) (*chain.Chain, error) {
	if chain, parseErr := chain.Import(cfg.GenesisPath); parseErr != nil {
		return nil, parseErr
	} else {
		return chain, nil
	}
}

// Parsing the prometheus address from the configuration file.
// In case that prometheus address is not defined, won't parse the address.
func ParsePrometheusAddress(cfg *config.Config) (*net.TCPAddr, error) {
	if cfg.Telemetry == nil || cfg.Telemetry.PrometheusAddr == "" {
		return nil, nil
	}

	return helper.ResolveAddr(
		cfg.Telemetry.PrometheusAddr,
		helper.AllInterfacesBinding,
	)
}

func ParseGrpcAddress(cfg *config.Config) (*net.TCPAddr, error) {
	return helper.ResolveAddr(
		cfg.GRPCAddr,
		helper.LocalHostBinding,
	)
}

func ParseLibp2pAddress(cfg *config.Config) (*net.TCPAddr, error) {
	return helper.ResolveAddr(
		cfg.Network.Libp2pAddr,
		helper.LocalHostBinding,
	)
}

func ParseJsonRpcAddress(cfg *config.Config) (*net.TCPAddr, error) {
	return helper.ResolveAddr(
		cfg.JSONRPCAddr,
		helper.AllInterfacesBinding,
	)
}

func ParseNatAddress(cfg *config.Config) (net.IP, error) {
	if cfg.Network.NatAddr == "" {
		return nil, nil
	}

	addr := net.ParseIP(cfg.Network.NatAddr)
	if addr == nil {
		return nil, errors.New("invalid network nat address provided")
	}
	return addr, nil
}

func ParseDNSAddress(cfg *config.Config, p2pPort int) (multiaddr.Multiaddr, error) {
	if cfg.Network.DNSAddr == "" {
		return nil, nil
	}

	return common.MultiAddrFromDNS(
		cfg.Network.DNSAddr, p2pPort,
	)
}

func ParseSecretsConfig(cfg *config.Config) (*secrets.SecretsManagerConfig, error) {
	if cfg.SecretsConfigPath == "" {
		return nil, nil
	}

	return secrets.ReadConfig(cfg.SecretsConfigPath)
}