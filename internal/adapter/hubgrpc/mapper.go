package hubgrpc

import (
	"time"

	"github.com/gitopshq-io/agent/internal/domain"
	agentv1 "github.com/gitopshq-io/agent/proto/agent/v1"
)

func toProtoClusterInfo(cluster domain.Cluster) agentv1.ClusterInfo {
	return agentv1.ClusterInfo{
		Name:              cluster.Name,
		DisplayName:       cluster.DisplayName,
		Provider:          cluster.Provider,
		Region:            cluster.Region,
		Environment:       cluster.Environment,
		AgentVersion:      cluster.AgentVersion,
		KubernetesVersion: cluster.KubernetesVersion,
		Capabilities:      toProtoCapabilities(cluster.Capabilities),
		Metadata:          cluster.Metadata,
	}
}

func fromProtoRegisterResponse(resp *agentv1.RegisterResponse) domain.RegisterResponse {
	if resp == nil {
		return domain.RegisterResponse{}
	}
	return domain.RegisterResponse{
		AgentToken:     resp.AgentToken,
		ClusterID:      resp.ClusterID,
		StatusInterval: time.Duration(resp.StatusIntervalSeconds) * time.Second,
	}
}

func toProtoAgentEnvelope(message domain.AgentMessage) *agentv1.AgentEnvelope {
	return &agentv1.AgentEnvelope{
		Heartbeat:             toProtoHeartbeat(message.Heartbeat),
		InventorySnapshot:     toProtoInventorySnapshot(message.InventorySnapshot),
		ArgoApplicationStatus: toProtoApplicationStatus(message.ApplicationStatus),
		DriftReport:           toProtoDriftReport(message.DriftReport),
		CommandAck:            toProtoCommandAck(message.CommandAck),
		CommandResult:         toProtoCommandResult(message.CommandResult),
		CredentialSyncResult:  toProtoCredentialSyncResult(message.CredentialSyncResult),
	}
}

func fromProtoHubEnvelope(message *agentv1.HubEnvelope) domain.HubMessage {
	if message == nil {
		return domain.HubMessage{}
	}
	return domain.HubMessage{
		ExecuteCommand:  fromProtoExecuteCommand(message.ExecuteCommand),
		SyncCredentials: fromProtoCredentialSyncRequest(message.SyncCredentials),
		RotateToken:     fromProtoRotateToken(message.RotateToken),
		ConfigUpdate:    fromProtoConfigUpdate(message.ConfigUpdate),
		Ping:            fromProtoPing(message.Ping),
	}
}

func toProtoCapabilities(capabilities []domain.Capability) []agentv1.Capability {
	out := make([]agentv1.Capability, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, agentv1.Capability(capability))
	}
	return out
}

func fromProtoCapabilities(capabilities []agentv1.Capability) []domain.Capability {
	out := make([]domain.Capability, 0, len(capabilities))
	for _, capability := range capabilities {
		out = append(out, domain.Capability(capability))
	}
	return out
}

func toProtoHeartbeat(heartbeat *domain.Heartbeat) *agentv1.Heartbeat {
	if heartbeat == nil {
		return nil
	}
	return &agentv1.Heartbeat{
		ClusterID:    heartbeat.ClusterID,
		AgentVersion: heartbeat.AgentVersion,
		Capabilities: toProtoCapabilities(heartbeat.Capabilities),
		Timestamp:    heartbeat.Timestamp,
	}
}

func toProtoInventorySnapshot(snapshot *domain.InventorySnapshot) *agentv1.InventorySnapshot {
	if snapshot == nil {
		return nil
	}
	out := &agentv1.InventorySnapshot{
		Timestamp: snapshot.Timestamp,
		Summary: agentv1.InventorySummary{
			ClusterName:       snapshot.Summary.ClusterName,
			NamespaceCount:    snapshot.Summary.NamespaceCount,
			NodeCount:         snapshot.Summary.NodeCount,
			ReadyNodeCount:    snapshot.Summary.ReadyNodeCount,
			PodCount:          snapshot.Summary.PodCount,
			DeploymentCount:   snapshot.Summary.DeploymentCount,
			KubernetesVersion: snapshot.Summary.KubernetesVersion,
		},
	}
	for _, resource := range snapshot.Resources {
		out.Resources = append(out.Resources, agentv1.ResourceRef{
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
			Status:    resource.Status,
			Labels:    resource.Labels,
		})
	}
	return out
}

func toProtoApplicationStatus(status *domain.ArgoApplicationStatus) *agentv1.ArgoApplicationStatus {
	if status == nil {
		return nil
	}
	out := &agentv1.ArgoApplicationStatus{Timestamp: status.Timestamp}
	for _, application := range status.Applications {
		out.Applications = append(out.Applications, agentv1.ArgoApplication{
			Name:                   application.Name,
			Namespace:              application.Namespace,
			Project:                application.Project,
			RepoURL:                application.RepoURL,
			Path:                   application.Path,
			TargetRevision:         application.TargetRevision,
			SyncStatus:             application.SyncStatus,
			HealthStatus:           application.HealthStatus,
			ResourceCount:          application.ResourceCount,
			LastSyncedAt:           application.LastSyncedAt,
			DestinationServer:      application.DestinationServer,
			DestinationNamespace:   application.DestinationNamespace,
			SourceType:             application.SourceType,
			Sources:                toProtoArgoSources(application.Sources),
			LiveRevision:           application.LiveRevision,
			ReconciledAt:           application.ReconciledAt,
			OutOfSyncResourceCount: application.OutOfSyncResourceCount,
			DegradedResourceCount:  application.DegradedResourceCount,
			SyncPolicy:             toProtoArgoSyncPolicy(application.SyncPolicy),
			Operation:              toProtoArgoOperationState(application.Operation),
			Conditions:             toProtoArgoConditions(application.Conditions),
			Resources:              toProtoArgoResources(application.Resources),
			Events:                 toProtoArgoEvents(application.Events),
			History:                toProtoArgoHistory(application.History),
		})
	}
	return out
}

func toProtoArgoSources(sources []domain.ArgoSource) []agentv1.ArgoSource {
	out := make([]agentv1.ArgoSource, 0, len(sources))
	for _, source := range sources {
		out = append(out, agentv1.ArgoSource{
			RepoURL:        source.RepoURL,
			Path:           source.Path,
			Chart:          source.Chart,
			TargetRevision: source.TargetRevision,
			Ref:            source.Ref,
			Type:           source.Type,
		})
	}
	return out
}

func toProtoArgoSyncPolicy(policy *domain.ArgoSyncPolicy) *agentv1.ArgoSyncPolicy {
	if policy == nil {
		return nil
	}
	return &agentv1.ArgoSyncPolicy{
		Automated:  policy.Automated,
		Prune:      policy.Prune,
		SelfHeal:   policy.SelfHeal,
		AllowEmpty: policy.AllowEmpty,
	}
}

func toProtoArgoOperationState(operation *domain.ArgoOperationState) *agentv1.ArgoOperationState {
	if operation == nil {
		return nil
	}
	return &agentv1.ArgoOperationState{
		Phase:      operation.Phase,
		Message:    operation.Message,
		StartedAt:  operation.StartedAt,
		FinishedAt: operation.FinishedAt,
	}
}

func toProtoArgoConditions(conditions []domain.ArgoCondition) []agentv1.ArgoCondition {
	out := make([]agentv1.ArgoCondition, 0, len(conditions))
	for _, condition := range conditions {
		out = append(out, agentv1.ArgoCondition{
			Type:             condition.Type,
			Message:          condition.Message,
			LastTransitionAt: condition.LastTransitionAt,
		})
	}
	return out
}

func toProtoArgoResources(resources []domain.ArgoResource) []agentv1.ArgoResource {
	out := make([]agentv1.ArgoResource, 0, len(resources))
	for _, resource := range resources {
		out = append(out, agentv1.ArgoResource{
			Group:           resource.Group,
			Kind:            resource.Kind,
			Namespace:       resource.Namespace,
			Name:            resource.Name,
			Version:         resource.Version,
			SyncStatus:      resource.SyncStatus,
			HealthStatus:    resource.HealthStatus,
			HealthMessage:   resource.HealthMessage,
			HookType:        resource.HookType,
			RequiresPruning: resource.RequiresPruning,
		})
	}
	return out
}

func toProtoArgoEvents(events []domain.ArgoEvent) []agentv1.ArgoEvent {
	out := make([]agentv1.ArgoEvent, 0, len(events))
	for _, event := range events {
		out = append(out, agentv1.ArgoEvent{
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			Namespace:      event.Namespace,
			Kind:           event.Kind,
			Name:           event.Name,
			Count:          event.Count,
			FirstTimestamp: event.FirstTimestamp,
			LastTimestamp:  event.LastTimestamp,
		})
	}
	return out
}

func toProtoArgoHistory(history []domain.ArgoHistoryEntry) []agentv1.ArgoHistoryEntry {
	out := make([]agentv1.ArgoHistoryEntry, 0, len(history))
	for _, entry := range history {
		out = append(out, agentv1.ArgoHistoryEntry{
			ID:         entry.ID,
			Revision:   entry.Revision,
			DeployedAt: entry.DeployedAt,
			SourceType: entry.SourceType,
		})
	}
	return out
}

func toProtoDriftReport(report *domain.DriftReport) *agentv1.DriftReport {
	if report == nil {
		return nil
	}
	out := &agentv1.DriftReport{Timestamp: report.Timestamp}
	for _, finding := range report.Findings {
		out.Findings = append(out.Findings, agentv1.DriftFinding{
			ID:             finding.ID,
			Severity:       finding.Severity,
			Scope:          finding.Scope,
			Kind:           finding.Kind,
			Namespace:      finding.Namespace,
			Name:           finding.Name,
			Summary:        finding.Summary,
			DesiredVersion: finding.DesiredVersion,
			LiveVersion:    finding.LiveVersion,
			DetectedAt:     finding.DetectedAt,
			Details:        finding.Details,
		})
	}
	return out
}

func toProtoCommandAck(ack *domain.CommandAck) *agentv1.CommandAck {
	if ack == nil {
		return nil
	}
	return &agentv1.CommandAck{
		CommandID: ack.CommandID,
		Status:    agentv1.CommandStatus(ack.Status),
		Message:   ack.Message,
		Timestamp: ack.Timestamp,
	}
}

func toProtoCommandResult(result *domain.CommandResult) *agentv1.CommandResult {
	if result == nil {
		return nil
	}
	return &agentv1.CommandResult{
		CommandID: result.CommandID,
		Status:    agentv1.CommandStatus(result.Status),
		Message:   result.Message,
		Error:     result.Error,
		Result:    result.Result,
		Timestamp: result.Timestamp,
	}
}

func toProtoCredentialSyncResult(result *domain.CredentialSyncResult) *agentv1.CredentialSyncResult {
	if result == nil {
		return nil
	}
	return &agentv1.CredentialSyncResult{
		Version:    result.Version,
		Status:     result.Status,
		Message:    result.Message,
		Namespace:  result.Namespace,
		SecretName: result.SecretName,
		Timestamp:  result.Timestamp,
	}
}

func fromProtoExecuteCommand(command *agentv1.ExecuteCommand) *domain.ExecuteCommand {
	if command == nil {
		return nil
	}
	return &domain.ExecuteCommand{
		CommandID:           command.CommandID,
		RequiredCapability:  domain.Capability(command.RequiredCapability),
		ExpiresAt:           command.ExpiresAt,
		SpecHash:            command.SpecHash,
		RequestedBy:         command.RequestedBy,
		ArgoSync:            fromProtoArgoSyncCommand(command.ArgoSync),
		ArgoRollback:        fromProtoArgoRollbackCommand(command.ArgoRollback),
		ArgoDelete:          fromProtoArgoDeleteCommand(command.ArgoDelete),
		DeployHelmRelease:   fromProtoDeployHelmReleaseCommand(command.DeployHelmRelease),
		ApplyKustomize:      fromProtoApplyKustomizeCommand(command.ApplyKustomize),
		ApplyManifestBundle: fromProtoApplyManifestBundleCommand(command.ApplyManifestBundle),
		RestartWorkload:     fromProtoRestartWorkloadCommand(command.RestartWorkload),
		ScaleWorkload:       fromProtoScaleWorkloadCommand(command.ScaleWorkload),
		RunDriftScan:        fromProtoRunDriftScanCommand(command.RunDriftScan),
		InspectResource:     fromProtoInspectResourceCommand(command.InspectResource),
	}
}

func toProtoExecuteCommand(command *domain.ExecuteCommand) *agentv1.ExecuteCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ExecuteCommand{
		CommandID:           command.CommandID,
		RequiredCapability:  agentv1.Capability(command.RequiredCapability),
		ExpiresAt:           command.ExpiresAt,
		SpecHash:            command.SpecHash,
		RequestedBy:         command.RequestedBy,
		ArgoSync:            toProtoArgoSyncCommand(command.ArgoSync),
		ArgoRollback:        toProtoArgoRollbackCommand(command.ArgoRollback),
		ArgoDelete:          toProtoArgoDeleteCommand(command.ArgoDelete),
		DeployHelmRelease:   toProtoDeployHelmReleaseCommand(command.DeployHelmRelease),
		ApplyKustomize:      toProtoApplyKustomizeCommand(command.ApplyKustomize),
		ApplyManifestBundle: toProtoApplyManifestBundleCommand(command.ApplyManifestBundle),
		RestartWorkload:     toProtoRestartWorkloadCommand(command.RestartWorkload),
		ScaleWorkload:       toProtoScaleWorkloadCommand(command.ScaleWorkload),
		RunDriftScan:        toProtoRunDriftScanCommand(command.RunDriftScan),
		InspectResource:     toProtoInspectResourceCommand(command.InspectResource),
	}
}

func fromProtoCredentialSyncRequest(req *agentv1.SyncCredentials) *domain.CredentialSyncRequest {
	if req == nil {
		return nil
	}
	out := &domain.CredentialSyncRequest{
		CommandID: req.CommandID,
		Version:   req.Version,
	}
	for _, bundle := range req.Bundles {
		out.Bundles = append(out.Bundles, domain.CredentialBundle{
			Version:        bundle.Version,
			Namespace:      bundle.Namespace,
			SecretName:     bundle.SecretName,
			Type:           bundle.Type,
			StringData:     bundle.StringData,
			Labels:         bundle.Labels,
			Annotations:    bundle.Annotations,
			RequiredScopes: append([]string(nil), bundle.RequiredScopes...),
		})
	}
	return out
}

func fromProtoRotateToken(token *agentv1.RotateToken) *domain.RotateToken {
	if token == nil {
		return nil
	}
	return &domain.RotateToken{
		CommandID: token.CommandID,
		NewToken:  token.NewToken,
		Timestamp: token.Timestamp,
	}
}

func fromProtoConfigUpdate(update *agentv1.ConfigUpdate) *domain.ConfigUpdate {
	if update == nil {
		return nil
	}
	return &domain.ConfigUpdate{
		StatusInterval: time.Duration(update.StatusIntervalSeconds) * time.Second,
		Capabilities:   fromProtoCapabilities(update.Capabilities),
	}
}

func fromProtoPing(ping *agentv1.Ping) *domain.Ping {
	if ping == nil {
		return nil
	}
	return &domain.Ping{Timestamp: ping.Timestamp}
}

func toProtoArgoSyncCommand(command *domain.ArgoSyncCommand) *agentv1.ArgoSyncCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ArgoSyncCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		Project:     command.Project,
		Prune:       command.Prune,
		DryRun:      command.DryRun,
	}
}

func fromProtoArgoSyncCommand(command *agentv1.ArgoSyncCommand) *domain.ArgoSyncCommand {
	if command == nil {
		return nil
	}
	return &domain.ArgoSyncCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		Project:     command.Project,
		Prune:       command.Prune,
		DryRun:      command.DryRun,
	}
}

func toProtoArgoRollbackCommand(command *domain.ArgoRollbackCommand) *agentv1.ArgoRollbackCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ArgoRollbackCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		ID:          command.ID,
		Prune:       command.Prune,
	}
}

func fromProtoArgoRollbackCommand(command *agentv1.ArgoRollbackCommand) *domain.ArgoRollbackCommand {
	if command == nil {
		return nil
	}
	return &domain.ArgoRollbackCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		ID:          command.ID,
		Prune:       command.Prune,
	}
}

func toProtoArgoDeleteCommand(command *domain.ArgoDeleteCommand) *agentv1.ArgoDeleteCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ArgoDeleteCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		Cascade:     command.Cascade,
	}
}

func fromProtoArgoDeleteCommand(command *agentv1.ArgoDeleteCommand) *domain.ArgoDeleteCommand {
	if command == nil {
		return nil
	}
	return &domain.ArgoDeleteCommand{
		Application: command.Application,
		Namespace:   command.Namespace,
		Cascade:     command.Cascade,
	}
}

func toProtoDeployHelmReleaseCommand(command *domain.DeployHelmReleaseCommand) *agentv1.DeployHelmReleaseCommand {
	if command == nil {
		return nil
	}
	return &agentv1.DeployHelmReleaseCommand{
		ReleaseName: command.ReleaseName,
		Namespace:   command.Namespace,
		Source:      toProtoSourceRef(command.Source),
		Values:      toProtoValuesRef(command.Values),
	}
}

func fromProtoDeployHelmReleaseCommand(command *agentv1.DeployHelmReleaseCommand) *domain.DeployHelmReleaseCommand {
	if command == nil {
		return nil
	}
	return &domain.DeployHelmReleaseCommand{
		ReleaseName: command.ReleaseName,
		Namespace:   command.Namespace,
		Source:      fromProtoSourceRef(command.Source),
		Values:      fromProtoValuesRef(command.Values),
	}
}

func toProtoApplyKustomizeCommand(command *domain.ApplyKustomizeCommand) *agentv1.ApplyKustomizeCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ApplyKustomizeCommand{
		Namespace: command.Namespace,
		Source:    toProtoSourceRef(command.Source),
		Values:    toProtoValuesRefPtr(command.Values),
	}
}

func fromProtoApplyKustomizeCommand(command *agentv1.ApplyKustomizeCommand) *domain.ApplyKustomizeCommand {
	if command == nil {
		return nil
	}
	var values *domain.ValuesRef
	if command.Values != nil {
		v := fromProtoValuesRef(*command.Values)
		values = &v
	}
	return &domain.ApplyKustomizeCommand{
		Namespace: command.Namespace,
		Source:    fromProtoSourceRef(command.Source),
		Values:    values,
	}
}

func toProtoApplyManifestBundleCommand(command *domain.ApplyManifestBundleCommand) *agentv1.ApplyManifestBundleCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ApplyManifestBundleCommand{
		Namespace: command.Namespace,
		Source:    toProtoSourceRef(command.Source),
	}
}

func fromProtoApplyManifestBundleCommand(command *agentv1.ApplyManifestBundleCommand) *domain.ApplyManifestBundleCommand {
	if command == nil {
		return nil
	}
	return &domain.ApplyManifestBundleCommand{
		Namespace: command.Namespace,
		Source:    fromProtoSourceRef(command.Source),
	}
}

func toProtoRestartWorkloadCommand(command *domain.RestartWorkloadCommand) *agentv1.RestartWorkloadCommand {
	if command == nil {
		return nil
	}
	return &agentv1.RestartWorkloadCommand{
		Namespace: command.Namespace,
		Kind:      command.Kind,
		Name:      command.Name,
	}
}

func fromProtoRestartWorkloadCommand(command *agentv1.RestartWorkloadCommand) *domain.RestartWorkloadCommand {
	if command == nil {
		return nil
	}
	return &domain.RestartWorkloadCommand{
		Namespace: command.Namespace,
		Kind:      command.Kind,
		Name:      command.Name,
	}
}

func toProtoScaleWorkloadCommand(command *domain.ScaleWorkloadCommand) *agentv1.ScaleWorkloadCommand {
	if command == nil {
		return nil
	}
	return &agentv1.ScaleWorkloadCommand{
		Namespace: command.Namespace,
		Kind:      command.Kind,
		Name:      command.Name,
		Replicas:  command.Replicas,
	}
}

func fromProtoScaleWorkloadCommand(command *agentv1.ScaleWorkloadCommand) *domain.ScaleWorkloadCommand {
	if command == nil {
		return nil
	}
	return &domain.ScaleWorkloadCommand{
		Namespace: command.Namespace,
		Kind:      command.Kind,
		Name:      command.Name,
		Replicas:  command.Replicas,
	}
}

func toProtoRunDriftScanCommand(command *domain.RunDriftScanCommand) *agentv1.RunDriftScanCommand {
	if command == nil {
		return nil
	}
	return &agentv1.RunDriftScanCommand{Scope: command.Scope}
}

func fromProtoRunDriftScanCommand(command *agentv1.RunDriftScanCommand) *domain.RunDriftScanCommand {
	if command == nil {
		return nil
	}
	return &domain.RunDriftScanCommand{Scope: command.Scope}
}

func toProtoInspectResourceCommand(command *domain.InspectResourceCommand) *agentv1.InspectResourceCommand {
	if command == nil {
		return nil
	}
	return &agentv1.InspectResourceCommand{
		Namespace:     command.Namespace,
		Kind:          command.Kind,
		Name:          command.Name,
		Container:     command.Container,
		TailLines:     command.TailLines,
		IncludeEvents: command.IncludeEvents,
		IncludeLogs:   command.IncludeLogs,
	}
}

func fromProtoInspectResourceCommand(command *agentv1.InspectResourceCommand) *domain.InspectResourceCommand {
	if command == nil {
		return nil
	}
	return &domain.InspectResourceCommand{
		Namespace:     command.Namespace,
		Kind:          command.Kind,
		Name:          command.Name,
		Container:     command.Container,
		TailLines:     command.TailLines,
		IncludeEvents: command.IncludeEvents,
		IncludeLogs:   command.IncludeLogs,
	}
}

func toProtoSourceRef(ref domain.SourceRef) agentv1.SourceRef {
	return agentv1.SourceRef{
		Type:             ref.Type,
		URL:              ref.URL,
		ResolvedRevision: ref.ResolvedRevision,
		ResolvedDigest:   ref.ResolvedDigest,
		Chart:            ref.Chart,
		Path:             ref.Path,
		CredentialRef:    toProtoCredentialRefPtr(ref.CredentialRef),
	}
}

func fromProtoSourceRef(ref agentv1.SourceRef) domain.SourceRef {
	return domain.SourceRef{
		Type:             ref.Type,
		URL:              ref.URL,
		ResolvedRevision: ref.ResolvedRevision,
		ResolvedDigest:   ref.ResolvedDigest,
		Chart:            ref.Chart,
		Path:             ref.Path,
		CredentialRef:    fromProtoCredentialRef(ref.CredentialRef),
	}
}

func toProtoValuesRef(ref domain.ValuesRef) agentv1.ValuesRef {
	return agentv1.ValuesRef{
		Digest:        ref.Digest,
		InlineValues:  ref.InlineValues,
		CredentialRef: toProtoCredentialRefPtr(ref.CredentialRef),
	}
}

func fromProtoValuesRef(ref agentv1.ValuesRef) domain.ValuesRef {
	return domain.ValuesRef{
		Digest:        ref.Digest,
		InlineValues:  ref.InlineValues,
		CredentialRef: fromProtoCredentialRef(ref.CredentialRef),
	}
}

func toProtoValuesRefPtr(ref *domain.ValuesRef) *agentv1.ValuesRef {
	if ref == nil {
		return nil
	}
	value := toProtoValuesRef(*ref)
	return &value
}

func toProtoCredentialRefPtr(ref *domain.CredentialRef) *agentv1.CredentialRef {
	if ref == nil {
		return nil
	}
	return &agentv1.CredentialRef{
		Namespace:  ref.Namespace,
		SecretName: ref.SecretName,
		Key:        ref.Key,
	}
}

func fromProtoCredentialRef(ref *agentv1.CredentialRef) *domain.CredentialRef {
	if ref == nil {
		return nil
	}
	return &domain.CredentialRef{
		Namespace:  ref.Namespace,
		SecretName: ref.SecretName,
		Key:        ref.Key,
	}
}
