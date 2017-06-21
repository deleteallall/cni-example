package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	cniVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

var (
	CNIVerion       = cniVersion.Current()
	MTU             = 1500
	ErrLinkNotFound = errors.New("link not found")
)

type CmdArgs struct {
	ContainerID string
	Netns       string
	IfName      string
	Args        string
	Path        string
	StdinData   []byte
}

func cmdAdd(args *skel.CmdArgs) error {
	prettyPrint("Begin to add")

	log.Println("ContainerID:", args.ContainerID)
	log.Println("Netns:", args.Netns)
	log.Println("IfName:", args.IfName)
	log.Println("Args:", args.Args)
	log.Println("Path:", args.Path)
	log.Println("StdinData:", string(args.StdinData))

	// 构建 veth pair
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	contIface := &current.Interface{}
	hostIface := &current.Interface{}

	// 在容器的 network namespace 下执行
	err = netns.Do(func(hostNS ns.NetNS) error {
		// create the veth pair in the container and move host end into host netns
		hostVeth, containerVeth, err := SetupVeth(args.IfName, MTU, hostNS)
		if err != nil {
			return err
		}

		contIface.Name = containerVeth.Name
		contIface.Mac = containerVeth.HardwareAddr.String()
		contIface.Sandbox = netns.Path()
		hostIface.Name = hostVeth.Name
		return nil
	})

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByName(hostIface.Name)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", hostIface.Name, err)
	}
	hostIface.Mac = hostVeth.Attrs().HardwareAddr.String()

	result := &current.Result{
		CNIVersion: CNIVerion,
		IPs: []*current.IPConfig{
			&current.IPConfig{
				Version: "4",
				Address: net.IPNet{
					IP:   net.ParseIP("10.200.1.1"),
					Mask: net.ParseIP("10.200.1.1").DefaultMask(),
				},
			},
		},
		Interfaces: []*current.Interface{
			hostIface,
			contIface,
		},
	}

	return types.PrintResult(result, CNIVerion)
}

func cmdDel(args *skel.CmdArgs) error {
	prettyPrint("Begin to delete")

	log.Println("ContainerID:", args.ContainerID)
	log.Println("Netns:", args.Netns)
	log.Println("IfName:", args.IfName)
	log.Println("Args:", args.Args)
	log.Println("Path:", args.Path)
	log.Println("StdinData:", string(args.StdinData))

	if args.Netns == "" {
		return nil
	}

	var ipn *net.IPNet
	err := ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		var err error
		ipn, err = DelLinkByNameAddr(args.IfName, netlink.FAMILY_V4)
		if err != nil && err == ErrLinkNotFound {
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}

	log.Printf("ipn: %v", ipn)

	prettyPrint("Successfully delete veth")

	return nil
}

func DelLinkByNameAddr(ifName string, family int) (*net.IPNet, error) {
	iface, err := netlink.LinkByName(ifName)
	if err != nil {
		if err != nil && err.Error() == "Link not found" {
			return nil, ErrLinkNotFound
		}
		return nil, fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	addrs, err := netlink.AddrList(iface, family)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("failed to get IP addresses for %q: %v", ifName, err)
	}

	if err = netlink.LinkDel(iface); err != nil {
		return nil, fmt.Errorf("failed to delete %q: %v", ifName, err)
	}

	return addrs[0].IPNet, nil
}

func prettyPrint(msg string) {
	log.Printf("======================= %s =======================", msg)
}

func SetupVeth(contVethName string, mtu int, hostNS ns.NetNS) (net.Interface, net.Interface, error) {
	hostVethName, contVeth, err := makeVeth(contVethName, mtu)
	if err != nil {
		return net.Interface{}, net.Interface{}, err
	}

	if err = netlink.LinkSetUp(contVeth); err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to set %q up: %v", contVethName, err)
	}

	// set contVeth IP
	contVethAddr, err := netlink.ParseAddr("10.200.1.2/24")
	if err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to parse addr 10.200.1.2/24: %s", err)
	}
	if err = netlink.AddrAdd(contVeth, contVethAddr); err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to set container veth addr: %s", err)
	}

	hostVeth, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to lookup %q: %v", hostVethName, err)
	}

	if err = netlink.LinkSetNsFd(hostVeth, int(hostNS.Fd())); err != nil {
		return net.Interface{}, net.Interface{}, fmt.Errorf("failed to move veth to host netns: %v", err)
	}

	err = hostNS.Do(func(_ ns.NetNS) error {
		hostVeth, err = netlink.LinkByName(hostVethName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q in %q: %v", hostVethName, hostNS.Path(), err)
		}

		hostVethAddr, err := netlink.ParseAddr("10.200.1.1/24")
		if err != nil {
			return fmt.Errorf("failed to parse host veth addr: %s", err)
		}
		if err = netlink.AddrAdd(hostVeth, hostVethAddr); err != nil {
			return fmt.Errorf("failed to set host veth addr: %s", err)
		}

		if err = netlink.LinkSetUp(hostVeth); err != nil {
			return fmt.Errorf("failed to set %q up: %v", hostVethName, err)
		}
		return nil
	})
	if err != nil {
		return net.Interface{}, net.Interface{}, err
	}
	return ifaceFromNetlinkLink(hostVeth), ifaceFromNetlinkLink(contVeth), nil
}

func makeVeth(name string, mtu int) (peerName string, veth netlink.Link, err error) {
	for i := 0; i < 10; i++ {
		peerName, err = RandomVethName()
		if err != nil {
			return
		}

		veth, err = makeVethPair(name, peerName, mtu)
		switch {
		case err == nil:
			return
		case os.IsExist(err):
			if peerExists(peerName) {
				continue
			}
			err = fmt.Errorf("container veth name provided (%v) already exists", name)
			return
		default:
			err = fmt.Errorf("failed to make veth pair: %v", err)
			return
		}
	}

	// should really never be hit
	err = fmt.Errorf("failed to find a unique veth name")
	return
}

func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Reader.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func makeVethPair(name, peer string, mtu int) (netlink.Link, error) {
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:  name,
			Flags: net.FlagUp,
			MTU:   mtu,
		},
		PeerName: peer,
	}
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	return veth, nil
}

func peerExists(name string) bool {
	if _, err := netlink.LinkByName(name); err != nil {
		return false
	}
	return true
}

func ifaceFromNetlinkLink(l netlink.Link) net.Interface {
	a := l.Attrs()
	return net.Interface{
		Index:        a.Index,
		MTU:          a.MTU,
		Name:         a.Name,
		HardwareAddr: a.HardwareAddr,
		Flags:        a.Flags,
	}
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, cniVersion.All)
}
