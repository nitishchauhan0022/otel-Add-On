package agent

import (
	"embed"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"k8s.io/client-go/kubernetes"
	"open-cluster-management.io/addon-framework/pkg/agent"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"open-cluster-management.io/cluster-proxy/pkg/config"
	"k8s.io/apimachinery/pkg/types"
	"open-cluster-management.io/addon-framework/pkg/utils"
	"k8s.io/client-go/rest"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"otel-add-on/pkg/common"

)

//go:embed manifests
var FS embed.FS


func NewAgentAddon(runtimeClient client.Client, nativeClient kubernetes.Interface) (agent.AgentAddon, error) {

	return addonfactory.NewAgentAddonFactory(common.AddonName, FS, "manifests/charts/addon-agent").
		WithAgentRegistrationOption(NewRegistrationOption()).
		WithInstallStrategy(agent.InstallAllStrategy(config.AddonInstallNamespace)).
		WithGetValuesFuncs(GetcollectorValueFunc(runtimeClient, nativeClient)).
		BuildHelmAgentAddon()

}

func NewRegistrationOption(kubeConfig *rest.Config, addonName, agentName string,nativeClient kubernetes.Interface) *agent.RegistrationOption {
	return &agent.RegistrationOption{
		CSRConfigurations: agent.KubeClientSignerConfigurations(addonName, agentName),
		CSRApproveCheck:   utils.DefaultCSRApprover(agentName),
		PermissionConfig: utils.NewRBACPermissionConfigBuilder(nativeClient).
				WithStaticRole(&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name: "otel-collector-addon-agent",
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"coordination.k8s.io"},
							Verbs:     []string{"*"},
							Resources: []string{"leases"},
						},
					},
				}).
				WithStaticRoleBinding(&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "otel-collector-addon-agent",
					},
					RoleRef: rbacv1.RoleRef{
						Kind: "Role",
						Name: "otel-collector-addon-agent",
					},
					Subjects: []rbacv1.Subject{
						{
							Kind: rbacv1.GroupKind,
							Name: common.SubjectGroupOtelCollector,
						},
					},
				}).Build(),
	}
}

func GetcollectorValueFunc(
	runtimeClient client.Client,
	nativeClient kubernetes.Interface) addonfactory.GetValuesFunc {
	return func(cluster *clusterv1.ManagedCluster,
		addon *addonv1alpha1.ManagedClusterAddOn) (addonfactory.Values, error) {

		// prepping
		clusterAddon := &addonv1alpha1.ClusterManagementAddOn{}
		if err := runtimeClient.Get(context.TODO(), types.NamespacedName{
			Name: common.AddonName,
		}, clusterAddon); err != nil {
			return nil, err
		}

		addonAgentArgs := []string{
			"--hub-kubeconfig=/etc/kubeconfig/kubeconfig",
			"--cluster-name=" + cluster.Name,
		}

		registry, image, tag, err := config.GetParsedAgentImage()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"agentDeploymentName":      "otel-collector-agent",
			"includeNamespaceCreation": true,
			"spokeAddonNamespace":      addon.Spec.InstallNamespace,
			// "additionalcollectorArgs":  "To be added",
			"clusterName":                   cluster.Name,
			"registry":                      registry,
			"image":                         image,
			"tag":                           tag,
			//"replicas":                      "To be added",
			"addonAgentArgs":                addonAgentArgs,
		}, nil
	}
}
