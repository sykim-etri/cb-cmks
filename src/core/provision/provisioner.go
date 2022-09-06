package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloud-barista/cb-mcks/src/core/app"
	"github.com/cloud-barista/cb-mcks/src/core/model"
	"github.com/cloud-barista/cb-mcks/src/core/tumblebug"
	"github.com/cloud-barista/cb-mcks/src/utils/lang"

	"golang.org/x/sync/errgroup"
)

/* new a instance of provider */
func NewProvisioner(cluster *model.Cluster) *Provisioner {
	provisioner := &Provisioner{
		Cluster:              cluster,
		WorkerNodeMachines:   make(map[string]*WorkerNodeMachine),
		ControlPlaneMachines: make(map[string]*ControlPlaneMachine),
	}
	if cluster.CpLeader != "" {
		for _, node := range cluster.Nodes {
			if node.Name == cluster.CpLeader {
				provisioner.leader = &ControlPlaneMachine{Machine: &Machine{
					Name:       node.Name,
					PublicIP:   node.PublicIP,
					PrivateIP:  node.PrivateIP,
					Username:   tumblebug.VM_USER_ACCOUNT,
					Credential: node.Credential,
					CSP:        node.Csp,
				}}
			}
		}
	}
	return provisioner
}

/* append a control-plane-machine */
func (self *Provisioner) AppendControlPlaneMachine(name string, csp app.CSP, region string, zone string, credential string) {

	machine := &ControlPlaneMachine{
		Machine: &Machine{
			Name:       name,
			CSP:        csp,
			Role:       app.CONTROL_PLANE,
			Region:     region,
			Zone:       zone,
			Credential: credential,
		},
	}
	self.ControlPlaneMachines[name] = machine
	if len(self.ControlPlaneMachines) == 1 {
		self.leader = machine
	}

}

/* append a worker-node-machine */
func (self *Provisioner) AppendWorkerNodeMachine(name string, csp app.CSP, region string, zone string, credential string) {
	self.WorkerNodeMachines[name] = &WorkerNodeMachine{
		Machine: &Machine{
			Name:       name,
			CSP:        csp,
			Role:       app.WORKER,
			Region:     region,
			Zone:       zone,
			Credential: credential,
		},
	}
}

/* set fileds each machines (public-ip, region, zone, spec, username) */
func (self *Provisioner) BindVM(vms []tumblebug.VM) ([]*model.Node, error) {

	nodes := []*model.Node{}
	for _, vm := range vms {

		// validate created vm
		if vm.Status == tumblebug.VMSTATUS_FAILED {
			status := app.Status{}
			if err := json.Unmarshal([]byte(vm.SystemMessage), &status); err != nil {
				status.Message = vm.SystemMessage
			}
			return nil, errors.New(fmt.Sprintf("Failed to create a vm (status=%s, cause='%s')", vm.Status, status.Message))
		} else if vm.PublicIP == "" && self.Cluster.ServiceType == app.ST_MULTI {
			return nil, errors.New(fmt.Sprintf("Failed to create a vm (status=%s, cause='unbounded public-ip')", vm.Status))
		} else if vm.PrivateIP == "" && self.Cluster.ServiceType == app.ST_SINGLE {
			return nil, errors.New(fmt.Sprintf("Failed to create a vm (status=%s, cause='unbounded private-ip')", vm.Status))
		}

		var machine *Machine

		if self.leader.Name == vm.Name {
			machine = self.leader.Machine
		} else {
			_, exists := self.ControlPlaneMachines[vm.Name]
			if exists {
				machine = self.ControlPlaneMachines[vm.Name].Machine
			} else {
				_, exists = self.WorkerNodeMachines[vm.Name]
				if exists {
					machine = self.WorkerNodeMachines[vm.Name].Machine
				}
			}
		}
		if machine != nil {
			machine.PublicIP = vm.PublicIP
			machine.PrivateIP = vm.PrivateIP
			machine.Username = vm.UserAccount
			machine.Region = lang.NVL(vm.Region.Region, machine.Region) // region, zone 공백인 경우가 간혹 있음
			machine.Zone = lang.NVL(vm.Region.Zone, machine.Zone)
			machine.Spec = vm.CspViewVmDetail.VMSpecName
			nodes = append(nodes, machine.NewNode())
			machine.FullName = ""
		} else {
			return nil, errors.New(fmt.Sprintf("Can't be found node by name '%s'", vm.Name))
		}
	}

	return nodes, nil
}

/* bootstrap */
func (self *Provisioner) Bootstrap() error {

	// bootstrap
	eg, _ := errgroup.WithContext(context.Background())

	for _, m := range self.GetMachinesAll() {
		machine := m
		eg.Go(func() error {
			if err := machine.ConnectionTest(); err != nil {
				return err
			}
			if err := machine.bootstrap(self.Cluster.NetworkCni, self.Cluster.Version, self.Cluster.ServiceType); err != nil {
				return err
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

/* setup haproxy */
func (self *Provisioner) InstallHAProxy() error {
	var servers string

	for _, machine := range self.ControlPlaneMachines {
		var err error
		var hostName string = machine.Name
		if self.Cluster.ServiceType == app.ST_SINGLE {
			if hostName, err = machine.GetFullName(machine.Name); err != nil {
				return err
			}
		}
		servers += fmt.Sprintf("  server  %s  %s:6443  check\\n", hostName, machine.PrivateIP)
	}
	if output, err := self.leader.executeSSH("sudo sed 's/^{{SERVERS}}/%s/g' %s/%s", servers, REMOTE_TARGET_PATH, "haproxy.sh"); err != nil {
		return err
	} else {
		if _, err = self.leader.executeSSH(output); err != nil {
			return err
		}
	}

	return nil
}

// control-plane init
func (self *Provisioner) InitControlPlane(kubernetesConfigReq app.ClusterConfigKubernetesReq) ([]string, string, error) {

	var joinCmd []string

	if output, err := self.leader.executeSSH("cd %s;./%s %s %s %s %s %s", REMOTE_TARGET_PATH, "k8s-init.sh", kubernetesConfigReq.PodCidr, kubernetesConfigReq.ServiceCidr, kubernetesConfigReq.ServiceDnsDomain, self.leader.PublicIP, self.leader.PrivateIP); err != nil {
		return nil, "", errors.New("Failed to initialize control-plane. (k8s-init.sh)")
	} else if strings.Contains(output, "Your Kubernetes control-plane has initialized successfully") {
		joinCmd = getJoinCmd(output)
	} else {
		return nil, "", errors.New("to initialize control-plane (the output not contains 'Your Kubernetes control-plane has initialized successfully')")
	}

	if self.Cluster.ServiceType == app.ST_SINGLE && len(kubernetesConfigReq.CloudConfig) > 0 {
		var contents string
		for _, keyValue := range kubernetesConfigReq.CloudConfig {
			contents += keyValue.Key + "=" + keyValue.Value + "\n"
		}

		if _, err := self.leader.executeSSH("cd %s;./%s %s $'%s'", REMOTE_TARGET_PATH, "gen-cloud-config.sh", self.leader.CSP, contents); err != nil {
			return nil, "", errors.New(fmt.Sprintf("Failed to initialize control-plane. (gen-cloud-config.sh, err=%v)", err))
		}

		self.leader.executeSSH("sudo cat %s/%s", REMOTE_TARGET_PATH, CCM_CLOUD_CONFIG_FILE)
	}

	ouput, _ := self.leader.executeSSH("sudo cat /etc/kubernetes/admin.conf")

	return joinCmd, ouput, nil
}

/* install network-cni */
func (self *Provisioner) InstallNetworkCni() error {

	cniYamls := []string{}
	if self.Cluster.NetworkCni == app.NETWORKCNI_CANAL {
		cniYamls = append(cniYamls, CNI_CANAL_FILE)
	} else if self.Cluster.NetworkCni == app.NETWORKCNI_KILO {
		cniYamls = append(cniYamls, CNI_KILO_FLANNEL_FILE)
		cniYamls = append(cniYamls, CNI_KILO_CRDS_FILE)
		cniYamls = append(cniYamls, CNI_KILO_KUBEADM_FILE)
	} else if self.Cluster.NetworkCni == app.NETWORKCNI_FLANNEL {
		cniYamls = append(cniYamls, CNI_FLANNEL_FILE)
	} else if self.Cluster.NetworkCni == app.NETWORKCNI_CALICO {
		cniYamls = append(cniYamls, CNI_CALICO_FILE)
	}

	for _, file := range cniYamls {
		if _, err := self.Kubectl("apply -f %s/%s", REMOTE_TARGET_PATH, file); err != nil {
			return err
		}
	}

	return nil
}

/* install cloud-controller-manager */
func (self *Provisioner) InstallCcm() error {

	ccmYamls := []string{}
	if self.leader.CSP == app.CSP_AWS {
		ccmYamls = append(ccmYamls, CCM_AWS_ROLE_SA_FILE)
		ccmYamls = append(ccmYamls, CCM_AWS_DS_FILE)

		if _, err := self.Kubectl("create secret -n kube-system generic cloud-config --from-file=cloud.conf=%s/%s", REMOTE_TARGET_PATH, CCM_CLOUD_CONFIG_FILE); err != nil {
			return err
		}

	} else if self.leader.CSP == app.CSP_OPENSTACK {
		ccmYamls = append(ccmYamls, CCM_OPENSTACK_ROLE_BINDINGS_FILE)
		ccmYamls = append(ccmYamls, CCM_OPENSTACK_ROLES_FILE)
		ccmYamls = append(ccmYamls, CCM_OPENSTACK_DS_FILE)

		if _, err := self.Kubectl("create secret -n kube-system generic cloud-config --from-file=cloud.conf=%s/%s", REMOTE_TARGET_PATH, CCM_CLOUD_CONFIG_FILE); err != nil {
			return err
		}
	}

	for _, file := range ccmYamls {
		if _, err := self.Kubectl("apply -f %s/%s", REMOTE_TARGET_PATH, file); err != nil {
			return err
		}
	}

	return nil
}

/* assign node labels */
func (self *Provisioner) AssignNodeLabelAnnotation() error {

	// commons labels
	for _, machine := range self.GetMachinesAll() {
		var err error
		var k8sNodeName string = machine.Name
		if self.Cluster.ServiceType == app.ST_SINGLE {
			if k8sNodeName, err = machine.GetFullName(machine.Name); err != nil {
				return err
			}
		}
		if _, err = self.Kubectl("label nodes %s %s=%s", k8sNodeName, app.LABEL_KEY_CSP, machine.CSP); err != nil {
			return err
		}
		if _, err = self.Kubectl("label nodes %s %s=%s", k8sNodeName, app.LABEL_KEY_REGION, machine.Region); err != nil {
			return err
		}
		if _, err = self.Kubectl("label nodes %s %s=%s", k8sNodeName, app.LABEL_KEY_ZONE, machine.Zone); err != nil {
			return err
		}
		if _, err = self.Kubectl("label nodes %s %s=%s", k8sNodeName, app.LABEL_KEY_CLUSTER, self.Cluster.Name); err != nil {
			return err
		}
	}

	// network-cni annotations
	if self.Cluster.NetworkCni == app.NETWORKCNI_KILO {
		for _, machine := range self.GetMachinesAll() {
			// use a full mesh network
			if _, err := self.Kubectl("annotate nodes %s kilo.squat.ai/location=%s", machine.Name, machine.Name); err != nil {
				return err
			}
			if _, err := self.Kubectl("annotate nodes %s kilo.squat.ai/persistent-keepalive=25", machine.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

/* new generate worker-node join command */
func (self *Provisioner) NewWorkerJoinCommand() (string, error) {

	if joinCommand, err := self.leader.executeSSH("sudo kubeadm token create --print-join-command"); err != nil {
		return "", err
	} else if joinCommand == "" {
		return "", errors.New("join command is empty")
	} else {
		return joinCommand, nil
	}
}

/* execute kubectl */
func (self *Provisioner) Kubectl(format string, a ...interface{}) (string, error) {

	command := fmt.Sprintf(format, a...)
	command = fmt.Sprintf("sudo kubectl %s --kubeconfig=/etc/kubernetes/admin.conf", command)
	if output, err := self.leader.executeSSH(command); err != nil {
		return "", errors.New(fmt.Sprintf("Failed to kubectl. (command='%s')", command))
	} else {
		return output, nil
	}

}

/* get machines */
func (self *Provisioner) GetMachinesAll() []*Machine {

	machines := []*Machine{}
	for _, m := range self.ControlPlaneMachines {
		machines = append(machines, m.Machine)
	}
	for _, m := range self.WorkerNodeMachines {
		machines = append(machines, m.Machine)
	}
	return machines
}

/* drain a node + delete node + delete a VM */
func (self *Provisioner) DrainAndDeleteNode(nodeName string) error {
	var k8sNodeName string = nodeName

	if self.Cluster.ServiceType == app.ST_SINGLE {
		var err error = nil
		for _, m := range self.GetMachinesAll() {
			if m != self.leader.Machine && m.Name == nodeName {
				if k8sNodeName, err = m.GetFullName(nodeName); err != nil {
					return errors.New(fmt.Sprintf("Failed to find a node (node=%s)", nodeName))
				}
				break
			}
		}
	}

	if output, err := self.Kubectl("drain %s --ignore-daemonsets --force --delete-local-data", k8sNodeName); err != nil {
		return errors.New(fmt.Sprintf("Failed to drain a node (node=%s, output='%s')", k8sNodeName, output))
	}
	if output, err := self.Kubectl("delete node %s", k8sNodeName); err != nil {
		return errors.New(fmt.Sprintf("Failed to delete a node (node=%s, output='%s')", k8sNodeName, output))
	}
	vm := tumblebug.NewVM(self.Cluster.Namespace, nodeName, self.Cluster.MCIS)
	if exists, err := vm.DELETE(); err != nil {
		return errors.New(fmt.Sprintf("Failed to remove a VM (%s)", vm.Name))
	} else if !exists {
		return errors.New(fmt.Sprintf("Failed to remove a VM (vm=%s, cause='Colud not be found a VM')", vm.Name))
	}

	return nil
}

func getJoinCmd(cpInitResult string) []string {
	var join1, join2, join3 string
	joinRegex, _ := regexp.Compile("kubeadm\\sjoin\\s(.*?)\\s--token\\s(.*?)\\n")
	joinRegex2, _ := regexp.Compile("--discovery-token-ca-cert-hash\\ssha256:(.*?)\\n")
	joinRegex3, _ := regexp.Compile("--control-plane --certificate-key(.*?)\\n")

	if joinRegex.MatchString(cpInitResult) {
		join1 = joinRegex.FindString(cpInitResult)
	}
	if joinRegex2.MatchString(cpInitResult) {
		join2 = joinRegex2.FindString(cpInitResult)
	}
	if joinRegex3.MatchString(cpInitResult) {
		join3 = joinRegex3.FindString(cpInitResult)
	}

	return []string{fmt.Sprintf("%s %s %s", join1, join2, join3), fmt.Sprintf("%s %s", join1, join2)}
}
