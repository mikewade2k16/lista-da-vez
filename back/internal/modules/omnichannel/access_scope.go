package omnichannel

import (
	"sort"
	"strings"
)

// InstanceAccessPolicy e a politica autoritativa de uma conexao. Novas conexoes e todo o
// backfill nascem RESTRICTED; ACCOUNT_SHARED so pode vir de uma escrita explicita e auditada.
type InstanceAccessPolicy string

const (
	InstanceAccessPolicyAccountShared InstanceAccessPolicy = "ACCOUNT_SHARED"
	InstanceAccessPolicyRestricted    InstanceAccessPolicy = "RESTRICTED"
)

// InstanceGrantLevel e hierarquico: manage inclui reply e view; reply inclui view.
type InstanceGrantLevel string

const (
	InstanceGrantView   InstanceGrantLevel = "view"
	InstanceGrantReply  InstanceGrantLevel = "reply"
	InstanceGrantManage InstanceGrantLevel = "manage"
)

type instanceFeaturePermissions struct {
	View         bool
	Reply        bool
	Assign       bool
	Close        bool
	Manage       bool
	ResetHistory bool
	Contacts     bool
	Settings     bool
	Agents       bool
	Audit        bool
}

// InstanceAccessDecision e a decisao relacional de uma instancia para o Principal atual.
// Reason e somente diagnostico interno e nunca deve ser serializado para o cliente.
type InstanceAccessDecision struct {
	InstanceID   string
	InstanceName string
	Policy       InstanceAccessPolicy
	GrantLevel   InstanceGrantLevel
	IsActive     bool
	Capabilities InstanceCapabilities
	Reason       string
}

// ConversationAccessScope e o objeto unico que o P1B passara a todas as superficies de
// conversa. O P1A o calcula em shadow mode, sem trocar ainda o filtro legado em producao.
type ConversationAccessScope struct {
	AccountID string
	UserID    string
	Eligible  bool
	Reason    string
	Instances map[string]InstanceAccessDecision
	features  instanceFeaturePermissions
}

func (s ConversationAccessScope) allowsPermission(key string) bool {
	switch strings.TrimSpace(key) {
	case "omnichannel.conversations.view":
		return s.features.View
	case "omnichannel.conversations.reply":
		return s.features.Reply
	case "omnichannel.conversations.assign":
		return s.features.Assign
	case "omnichannel.conversations.close":
		return s.features.Close
	case "omnichannel.instances.manage":
		return s.features.Manage
	case conversationPrivacyManagePermission:
		return s.features.ResetHistory
	case "omnichannel.contacts.manage":
		return s.features.Contacts
	case "omnichannel.settings.manage":
		return s.features.Settings
	case "omnichannel.agents.manage":
		return s.features.Agents
	case "omnichannel.audit.view":
		return s.features.Audit
	default:
		return false
	}
}

func (s ConversationAccessScope) instanceDecision(instanceID string) (InstanceAccessDecision, bool) {
	decision, ok := s.Instances[strings.TrimSpace(instanceID)]
	return decision, ok
}

func (s ConversationAccessScope) visibleInstanceIDs(required InstanceGrantLevel, activeOnly bool) []string {
	ids := make([]string, 0, len(s.Instances))
	for id, decision := range s.Instances {
		if activeOnly && !decision.IsActive {
			continue
		}
		if instanceCapabilityAllows(decision.Capabilities, required) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (s ConversationAccessScope) conversationVisibility(required InstanceGrantLevel) VisibilityScope {
	visibility := VisibilityScope{
		UserID: s.UserID,
	}
	for _, decision := range s.Instances {
		if !decision.IsActive || !instanceCapabilityAllows(decision.Capabilities, required) {
			continue
		}
		visibility.InstanceScopeKeys = append(visibility.InstanceScopeKeys, decision.InstanceName)
		// O grant relacional manage amplia o alcance das conversas da instancia.
		// A permission omnichannel.instances.manage continua exclusiva ao lifecycle/configuracao
		// e nao pode ser exigida para quem ja possui a feature de conversa solicitada.
		if instanceGrantAllows(decision.GrantLevel, InstanceGrantManage) {
			visibility.ManageInstanceScopeKeys = append(visibility.ManageInstanceScopeKeys, decision.InstanceName)
		}
	}
	sort.Strings(visibility.InstanceScopeKeys)
	sort.Strings(visibility.ManageInstanceScopeKeys)
	return visibility
}

func instanceCapabilityAllows(capabilities InstanceCapabilities, required InstanceGrantLevel) bool {
	switch required {
	case InstanceGrantManage:
		return capabilities.Manage
	case InstanceGrantReply:
		return capabilities.Reply
	case InstanceGrantView:
		return capabilities.View
	default:
		return false
	}
}

func validInstanceAccessPolicy(policy InstanceAccessPolicy) bool {
	switch policy {
	case InstanceAccessPolicyAccountShared, InstanceAccessPolicyRestricted:
		return true
	default:
		return false
	}
}

func validInstanceGrantLevel(level InstanceGrantLevel) bool {
	switch level {
	case InstanceGrantView, InstanceGrantReply, InstanceGrantManage:
		return true
	default:
		return false
	}
}

func instanceGrantRank(level InstanceGrantLevel) int {
	switch level {
	case InstanceGrantManage:
		return 3
	case InstanceGrantReply:
		return 2
	case InstanceGrantView:
		return 1
	default:
		return 0
	}
}

func instanceGrantAllows(level, required InstanceGrantLevel) bool {
	return instanceGrantRank(level) >= instanceGrantRank(required) && validInstanceGrantLevel(required)
}

func resolveInstanceCapabilities(policy InstanceAccessPolicy, grant InstanceGrantLevel, permissions instanceFeaturePermissions) (InstanceCapabilities, string) {
	dataView := policy == InstanceAccessPolicyAccountShared || instanceGrantAllows(grant, InstanceGrantView)
	dataReply := policy == InstanceAccessPolicyAccountShared || instanceGrantAllows(grant, InstanceGrantReply)
	dataManage := instanceGrantAllows(grant, InstanceGrantManage)

	capabilities := InstanceCapabilities{
		View:   permissions.View && dataView,
		Reply:  permissions.Reply && dataReply,
		Manage: permissions.Manage && dataManage,
	}
	capabilities.ResetHistory = capabilities.Manage && permissions.ResetHistory

	switch {
	case !permissions.View && !permissions.Reply && !permissions.Manage:
		return capabilities, "feature_permission_missing"
	case policy == InstanceAccessPolicyRestricted && strings.TrimSpace(string(grant)) == "":
		return capabilities, "instance_grant_missing"
	case permissions.Manage && !dataManage && !capabilities.View && !capabilities.Reply:
		return capabilities, "instance_manage_grant_missing"
	default:
		return capabilities, "allowed"
	}
}
