package bundle

import (
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func Plan(topology *model.Topology) model.Object {
	return model.Object{"topology": topology.Name, "runtime_version": xray.Version,
		"deployment_order": topology.Order, "nodes": topology.Nodes, "routes": topology.Routes,
		"limitations": []string{"Destination UDP/443 is blocked; UDP VPN listeners may use port 443", "One next hop per listener", "SSH settings are preserved"}}
}

func Render(topology *model.Topology, directory, installers string) error {
	for _, relative := range []string{"", "nodes", "clients", "installers"} {
		if err := safeio.PrivateDir(filepath.Join(directory, relative)); err != nil {
			return err
		}
	}
	lock, err := Lock(directory)
	if err != nil {
		return err
	}
	defer lock.Close()
	binaries, err := copyInstallers(installers, directory)
	if err != nil {
		return err
	}
	state, err := loadSecrets(topology, directory)
	if err != nil {
		return err
	}
	manifest := model.Deployment{SchemaVersion: 2, TopologyID: topology.Name, Order: topology.Order, Installers: binaries}
	manifest.Nodes, err = renderNodes(topology, directory, state)
	if err != nil {
		return err
	}
	manifest.ProfilesSHA256, err = renderClients(topology, directory, state)
	if err != nil {
		return err
	}
	if err := safeio.JSON(filepath.Join(directory, "deployment.json"), manifest); err != nil {
		return err
	}
	return safeio.JSON(filepath.Join(directory, "plan.json"), Plan(topology))
}
