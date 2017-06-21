package main

import (
	"fmt"
	"log"
	"net"
	"os/exec"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	cniVersion "github.com/containernetworking/cni/pkg/version"
)

const (
	CNIVerion = "0.2.0"

	BinAddPath = "/opt/cni/bin/example01-add.sh"
	BinDelPath = "/opt/cni/bin/example01-del.sh"
)

func cmdAdd(args *skel.CmdArgs) error {
	prettyPrint("Begin to add")

	log.Println("ContainerID:", args.ContainerID)
	log.Println("Netns:", args.Netns)
	log.Println("IfName:", args.IfName)
	log.Println("Args:", args.Args)
	log.Println("Path:", args.Path)
	log.Println("StdinData:", string(args.StdinData))

	contNS := args.Netns
	contID := args.ContainerID

	cmd := exec.Command(BinAddPath, contNS, contID)
	err := cmd.Run()

	if err != nil {
		prettyPrint(fmt.Sprintf("Failed to setup veth: %s", err))
		return err
	}

	prettyPrint("Successfully setup veth")

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

	cmd := exec.Command(BinDelPath)
	err := cmd.Run()

	if err != nil {
		prettyPrint(fmt.Sprintf("Failed to delete veth: %s", err))
		return err
	}

	prettyPrint("Successfully delete veth")

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
	}

	return types.PrintResult(result, CNIVerion)
}

func prettyPrint(msg string) {
	log.Printf("======================= %s =======================", msg)
}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, cniVersion.All)
}
