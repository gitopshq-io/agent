package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gitopshq-io/agent/internal/domain"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const identitySecretKey = "identity.json"

type SecretIdentityStore struct {
	Client     kubernetes.Interface
	Namespace  string
	SecretName string
}

func (s SecretIdentityStore) Load(ctx context.Context) (domain.AgentIdentity, error) {
	if s.Client == nil {
		return domain.AgentIdentity{}, errors.New("kubernetes client is required")
	}
	secret, err := s.Client.CoreV1().Secrets(s.Namespace).Get(ctx, s.SecretName, metav1.GetOptions{})
	if err != nil {
		return domain.AgentIdentity{}, err
	}
	payload, ok := secret.Data[identitySecretKey]
	if !ok || len(payload) == 0 {
		return domain.AgentIdentity{}, fmt.Errorf("secret %s/%s does not contain %s", s.Namespace, s.SecretName, identitySecretKey)
	}
	return decodeIdentity(payload), nil
}

func (s SecretIdentityStore) Save(ctx context.Context, identity domain.AgentIdentity) error {
	if s.Client == nil {
		return errors.New("kubernetes client is required")
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		return fmt.Errorf("marshal identity: %w", err)
	}
	secrets := s.Client.CoreV1().Secrets(s.Namespace)
	existing, err := secrets.Get(ctx, s.SecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = secrets.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.SecretName,
				Namespace: s.Namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "gitopshq-agent",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				identitySecretKey: payload,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create identity secret %s/%s: %w", s.Namespace, s.SecretName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get identity secret %s/%s: %w", s.Namespace, s.SecretName, err)
	}
	if existing.Data == nil {
		existing.Data = make(map[string][]byte, 1)
	}
	existing.Data[identitySecretKey] = payload
	if _, err := secrets.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update identity secret %s/%s: %w", s.Namespace, s.SecretName, err)
	}
	return nil
}

func decodeIdentity(payload []byte) domain.AgentIdentity {
	var identity domain.AgentIdentity
	if err := json.Unmarshal(payload, &identity); err == nil && identity.AgentToken != "" {
		return identity
	}
	return domain.AgentIdentity{AgentToken: strings.TrimSpace(string(payload))}
}
