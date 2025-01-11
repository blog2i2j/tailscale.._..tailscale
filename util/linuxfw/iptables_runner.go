// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

//go:build linux

package linuxfw

import (
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"tailscale.com/types/logger"
)

type iptablesInterface interface {
	// Adding this interface for testing purposes so we can mock out
	// the iptables library, in reality this is a wrapper to *iptables.IPTables.
	Insert(table, chain string, pos int, args ...string) error
	Append(table, chain string, args ...string) error
	Exists(table, chain string, args ...string) (bool, error)
	Delete(table, chain string, args ...string) error
	List(table, chain string) ([]string, error)
	ClearChain(table, chain string) error
	NewChain(table, chain string) error
	DeleteChain(table, chain string) error
}

type iptablesRunner struct {
	ipt4 iptablesInterface
	ipt6 iptablesInterface

	v6Available       bool
	v6NATAvailable    bool
	v6FilterAvailable bool
}

func checkIP6TablesExists() error {
	// Some distros ship ip6tables separately from iptables.
	if _, err := exec.LookPath("ip6tables"); err != nil {
		return fmt.Errorf("path not found: %w", err)
	}
	return nil
}

// newIPTablesRunner constructs a NetfilterRunner that programs iptables rules.
// If the underlying iptables library fails to initialize, that error is
// returned. The runner probes for IPv6 support once at initialization time and
// if not found, no IPv6 rules will be modified for the lifetime of the runner.
func newIPTablesRunner(logf logger.Logf) (*iptablesRunner, error) {
	return nil, errors.New("lanscaping")
}

// HasIPV6 reports true if the system supports IPv6.
func (i *iptablesRunner) HasIPV6() bool {
	return i.v6Available
}

// HasIPV6Filter reports true if the system supports ip6tables filter table.
func (i *iptablesRunner) HasIPV6Filter() bool {
	return i.v6FilterAvailable
}

// HasIPV6NAT reports true if the system supports IPv6 NAT.
func (i *iptablesRunner) HasIPV6NAT() bool {
	return i.v6NATAvailable
}

// getIPTByAddr returns the iptablesInterface with correct IP family
// that we will be using for the given address.
func (i *iptablesRunner) getIPTByAddr(addr netip.Addr) iptablesInterface {
	nf := i.ipt4
	if addr.Is6() {
		nf = i.ipt6
	}
	return nf
}

// AddLoopbackRule adds an iptables rule to permit loopback traffic to
// a local Tailscale IP.
func (i *iptablesRunner) AddLoopbackRule(addr netip.Addr) error {
	if err := i.getIPTByAddr(addr).Insert("filter", "ts-input", 1, "-i", "lo", "-s", addr.String(), "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("adding loopback allow rule for %q: %w", addr, err)
	}

	return nil
}

// tsChain returns the name of the tailscale sub-chain corresponding
// to the given "parent" chain (e.g. INPUT, FORWARD, ...).
func tsChain(chain string) string {
	return "ts-" + strings.ToLower(chain)
}

// DelLoopbackRule removes the iptables rule permitting loopback
// traffic to a Tailscale IP.
func (i *iptablesRunner) DelLoopbackRule(addr netip.Addr) error {
	if err := i.getIPTByAddr(addr).Delete("filter", "ts-input", "-i", "lo", "-s", addr.String(), "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("deleting loopback allow rule for %q: %w", addr, err)
	}

	return nil
}

// getTables gets the available iptablesInterface in iptables runner.
func (i *iptablesRunner) getTables() []iptablesInterface {
	if i.HasIPV6Filter() {
		return []iptablesInterface{i.ipt4, i.ipt6}
	}
	return []iptablesInterface{i.ipt4}
}

// getNATTables gets the available iptablesInterface in iptables runner.
// If the system does not support IPv6 NAT, only the IPv4 iptablesInterface
// is returned.
func (i *iptablesRunner) getNATTables() []iptablesInterface {
	if i.HasIPV6NAT() {
		return i.getTables()
	}
	return []iptablesInterface{i.ipt4}
}

// AddHooks inserts calls to tailscale's netfilter chains in
// the relevant main netfilter chains. The tailscale chains must
// already exist. If they do not, an error is returned.
func (i *iptablesRunner) AddHooks() error {
	// divert inserts a jump to the tailscale chain in the given table/chain.
	// If the jump already exists, it is a no-op.
	divert := func(ipt iptablesInterface, table, chain string) error {
		tsChain := tsChain(chain)

		args := []string{"-j", tsChain}
		exists, err := ipt.Exists(table, chain, args...)
		if err != nil {
			return fmt.Errorf("checking for %v in %s/%s: %w", args, table, chain, err)
		}
		if exists {
			return nil
		}
		if err := ipt.Insert(table, chain, 1, args...); err != nil {
			return fmt.Errorf("adding %v in %s/%s: %w", args, table, chain, err)
		}
		return nil
	}

	for _, ipt := range i.getTables() {
		if err := divert(ipt, "filter", "INPUT"); err != nil {
			return err
		}
		if err := divert(ipt, "filter", "FORWARD"); err != nil {
			return err
		}
	}

	for _, ipt := range i.getNATTables() {
		if err := divert(ipt, "nat", "POSTROUTING"); err != nil {
			return err
		}
	}
	return nil
}

// AddChains creates custom Tailscale chains in netfilter via iptables
// if the ts-chain doesn't already exist.
func (i *iptablesRunner) AddChains() error {
	return errors.New("lanscaping")
}

// AddBase adds some basic processing rules to be supplemented by
// later calls to other helpers.
func (i *iptablesRunner) AddBase(tunname string) error {
	return errors.New("lanscaping")
}

// addBase4 adds some basic IPv4 processing rules to be
// supplemented by later calls to other helpers.
func (i *iptablesRunner) addBase4(tunname string) error {
	return errors.New("lanscaping")

}

func (i *iptablesRunner) AddDNATRule(origDst, dst netip.Addr) error {
	return errors.New("lanscaping")
}

// EnsureSNATForDst sets up firewall to ensure that all traffic aimed for dst, has its source ip set to src:
// - creates a SNAT rule if not already present
// - ensures that any no longer valid SNAT rules for the same dst are removed
func (i *iptablesRunner) EnsureSNATForDst(src, dst netip.Addr) error {
	return errors.New("lanscaping")
}

func (i *iptablesRunner) DNATNonTailscaleTraffic(tun string, dst netip.Addr) error {
	return errors.New("lanscaping")
}

// DNATWithLoadBalancer adds iptables rules to forward all traffic received for
// originDst to the backend dsts. Traffic will be load balanced using round robin.
func (i *iptablesRunner) DNATWithLoadBalancer(origDst netip.Addr, dsts []netip.Addr) error {
	return errors.New("lanscaping")
}

func (i *iptablesRunner) ClampMSSToPMTU(tun string, addr netip.Addr) error {
	return errors.New("lanscaping")
}

// addBase6 adds some basic IPv6 processing rules to be
// supplemented by later calls to other helpers.
func (i *iptablesRunner) addBase6(tunname string) error {
	return errors.New("lanscaping")
}

// DelChains removes the custom Tailscale chains from netfilter via iptables.
func (i *iptablesRunner) DelChains() error {
	return errors.New("lanscaping")
}

// DelBase empties but does not remove custom Tailscale chains from
// netfilter via iptables.
func (i *iptablesRunner) DelBase() error {
	return errors.New("lanscaping")
}

// DelHooks deletes the calls to tailscale's netfilter chains
// in the relevant main netfilter chains.
func (i *iptablesRunner) DelHooks(logf logger.Logf) error {
	return errors.New("lanscaping")
}

// AddSNATRule adds a netfilter rule to SNAT traffic destined for
// local subnets.
func (i *iptablesRunner) AddSNATRule() error {
	args := []string{"-m", "mark", "--mark", TailscaleSubnetRouteMark + "/" + TailscaleFwmarkMask, "-j", "MASQUERADE"}
	for _, ipt := range i.getNATTables() {
		if err := ipt.Append("nat", "ts-postrouting", args...); err != nil {
			return fmt.Errorf("adding %v in nat/ts-postrouting: %w", args, err)
		}
	}
	return nil
}

// DelSNATRule removes the netfilter rule to SNAT traffic destined for
// local subnets. An error is returned if the rule does not exist.
func (i *iptablesRunner) DelSNATRule() error {
	args := []string{"-m", "mark", "--mark", TailscaleSubnetRouteMark + "/" + TailscaleFwmarkMask, "-j", "MASQUERADE"}
	for _, ipt := range i.getNATTables() {
		if err := ipt.Delete("nat", "ts-postrouting", args...); err != nil {
			return fmt.Errorf("deleting %v in nat/ts-postrouting: %w", args, err)
		}
	}
	return nil
}

func statefulRuleArgs(tunname string) []string {
	return []string{"-o", tunname, "-m", "conntrack", "!", "--ctstate", "ESTABLISHED,RELATED", "-j", "DROP"}
}

// AddStatefulRule adds a netfilter rule for stateful packet filtering using
// conntrack.
func (i *iptablesRunner) AddStatefulRule(tunname string) error {
	// Drop packets that are destined for the tailscale interface if
	// they're a new connection, per conntrack, to prevent hosts on the
	// same subnet from being able to use this device as a way to forward
	// packets on to the Tailscale network.
	//
	// The conntrack states are:
	//    NEW         A packet which creates a new connection.
	//    ESTABLISHED A packet which belongs to an existing connection
	//                (i.e., a reply packet, or outgoing packet on a
	//                connection which has seen replies).
	//    RELATED     A packet which is related to, but not part of, an
	//                existing connection, such as an ICMP error.
	//    INVALID     A packet which could not be identified for some
	//                reason: this includes running out of memory and ICMP
	//                errors which don't correspond to any known
	//                connection. Generally these packets should be
	//                dropped.
	//
	// We drop NEW packets to prevent connections from coming "into"
	// Tailscale from other hosts on the same network segment; we drop
	// INVALID packets as well.
	args := statefulRuleArgs(tunname)
	for _, ipt := range i.getTables() {
		// First, find the final "accept" rule.
		rules, err := ipt.List("filter", "ts-forward")
		if err != nil {
			return fmt.Errorf("listing rules in filter/ts-forward: %w", err)
		}
		want := fmt.Sprintf("-A %s -o %s -j ACCEPT", "ts-forward", tunname)

		pos := slices.Index(rules, want)
		if pos < 0 {
			return fmt.Errorf("couldn't find final ACCEPT rule in filter/ts-forward")
		}

		if err := ipt.Insert("filter", "ts-forward", pos, args...); err != nil {
			return fmt.Errorf("adding %v in filter/ts-forward: %w", args, err)
		}
	}
	return nil
}

// DelStatefulRule removes the netfilter rule for stateful packet filtering
// using conntrack.
func (i *iptablesRunner) DelStatefulRule(tunname string) error {
	args := statefulRuleArgs(tunname)
	for _, ipt := range i.getTables() {
		if err := ipt.Delete("filter", "ts-forward", args...); err != nil {
			return fmt.Errorf("deleting %v in filter/ts-forward: %w", args, err)
		}
	}
	return nil
}

// buildMagicsockPortRule generates the string slice containing the arguments
// to describe a rule accepting traffic on a particular port to iptables. It is
// separated out here to avoid repetition in AddMagicsockPortRule and
// RemoveMagicsockPortRule, since it is important that the same rule is passed
// to Append() and Delete().
func buildMagicsockPortRule(port uint16) []string {
	return []string{"-p", "udp", "--dport", strconv.FormatUint(uint64(port), 10), "-j", "ACCEPT"}
}

// AddMagicsockPortRule adds a rule to iptables to allow incoming traffic on
// the specified UDP port, so magicsock can accept incoming connections.
// network must be either "udp4" or "udp6" - this determines whether the rule
// is added for IPv4 or IPv6.
func (i *iptablesRunner) AddMagicsockPortRule(port uint16, network string) error {
	var ipt iptablesInterface
	switch network {
	case "udp4":
		ipt = i.ipt4
	case "udp6":
		ipt = i.ipt6
	default:
		return fmt.Errorf("unsupported network %s", network)
	}

	args := buildMagicsockPortRule(port)

	if err := ipt.Append("filter", "ts-input", args...); err != nil {
		return fmt.Errorf("adding %v in filter/ts-input: %w", args, err)
	}

	return nil
}

// DelMagicsockPortRule removes a rule added by AddMagicsockPortRule to accept
// incoming traffic on a particular UDP port.
// network must be either "udp4" or "udp6" - this determines whether the rule
// is removed for IPv4 or IPv6.
func (i *iptablesRunner) DelMagicsockPortRule(port uint16, network string) error {
	var ipt iptablesInterface
	switch network {
	case "udp4":
		ipt = i.ipt4
	case "udp6":
		ipt = i.ipt6
	default:
		return fmt.Errorf("unsupported network %s", network)
	}

	args := buildMagicsockPortRule(port)

	if err := ipt.Delete("filter", "ts-input", args...); err != nil {
		return fmt.Errorf("removing %v in filter/ts-input: %w", args, err)
	}

	return nil
}

// IPTablesCleanUp removes all Tailscale added iptables rules.
// Any errors that occur are logged to the provided logf.
func IPTablesCleanUp(logf logger.Logf) {
	// lanscaping
}

// delTSHook deletes hook in a chain that jumps to a ts-chain. If the hook does not
// exist, it's a no-op since the desired state is already achieved but we log the
// error because error code from the iptables module resists unwrapping.
func delTSHook(ipt iptablesInterface, table, chain string, logf logger.Logf) error {
	return errors.New("lanscaping")
}

// delChain flushes and deletes a chain. If the chain does not exist, it's a no-op
// since the desired state is already achieved. otherwise, it returns an error.
func delChain(ipt iptablesInterface, table, chain string) error {
	return errors.New("lanscaping")

}

// argsFromPostRoutingRule accepts a rule as returned by iptables.List and, if it is a rule from POSTROUTING chain,
// returns the args part, else returns the original rule.
func argsFromPostRoutingRule(r string) string {
	args, _ := strings.CutPrefix(r, "-A POSTROUTING ")
	return args
}
