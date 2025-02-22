/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package identityprovider

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-aws/pkg/cloud/services/wait"
)

var oidcType = aws.String("oidc")

type WaitIdentityProviderAssociatedProcedure struct {
	plan *plan
}

func (w *WaitIdentityProviderAssociatedProcedure) Name() string {
	return "wait_identity_provider_association"
}

func (w *WaitIdentityProviderAssociatedProcedure) Do(ctx context.Context) error {
	if err := wait.WaitForWithRetryable(wait.NewBackoff(), func() (bool, error) {
		out, err := w.plan.eksClient.DescribeIdentityProviderConfigWithContext(ctx, &eks.DescribeIdentityProviderConfigInput{
			ClusterName: aws.String(w.plan.clusterName),
			IdentityProviderConfig: &eks.IdentityProviderConfig{
				Name: aws.String(w.plan.currentIdentityProvider.IdentityProviderConfigName),
				Type: oidcType,
			},
		})

		if err != nil {
			return false, err
		}

		if aws.StringValue(out.IdentityProviderConfig.Oidc.Status) == eks.ConfigStatusActive {
			return true, nil
		}

		return false, nil
	}); err != nil {
		return errors.Wrap(err, "failed waiting for identity provider association to be ready")
	}

	return nil
}

type DisassociateIdentityProviderConfig struct {
	plan *plan
}

func (d *DisassociateIdentityProviderConfig) Name() string {
	return "dissociate_identity_provider"
}

func (d *DisassociateIdentityProviderConfig) Do(ctx context.Context) error {
	if err := wait.WaitForWithRetryable(wait.NewBackoff(), func() (bool, error) {
		_, err := d.plan.eksClient.DisassociateIdentityProviderConfigWithContext(ctx, &eks.DisassociateIdentityProviderConfigInput{
			ClusterName: aws.String(d.plan.clusterName),
			IdentityProviderConfig: &eks.IdentityProviderConfig{
				Name: aws.String(d.plan.currentIdentityProvider.IdentityProviderConfigName),
				Type: oidcType,
			},
		})

		if err != nil {
			return false, err
		}

		return true, nil
	}); err != nil {
		return errors.Wrap(err, "failing disassociating identity provider config")
	}

	return nil
}

type AssociateIdentityProviderProcedure struct {
	plan *plan
}

func (a *AssociateIdentityProviderProcedure) Name() string {
	return "associate_identity_provider"
}

func (a *AssociateIdentityProviderProcedure) Do(ctx context.Context) error {
	oidc := a.plan.desiredIdentityProvider
	input := &eks.AssociateIdentityProviderConfigInput{
		ClusterName: aws.String(a.plan.clusterName),
		Oidc: &eks.OidcIdentityProviderConfigRequest{
			ClientId:                   aws.String(oidc.ClientID),
			GroupsClaim:                oidc.GroupsClaim,
			GroupsPrefix:               oidc.GroupsPrefix,
			IdentityProviderConfigName: aws.String(oidc.IdentityProviderConfigName),
			IssuerUrl:                  aws.String(oidc.IssuerURL),
			RequiredClaims:             oidc.RequiredClaims,
			UsernameClaim:              oidc.UsernameClaim,
			UsernamePrefix:             oidc.UsernamePrefix,
		},
	}

	if len(oidc.Tags) > 0 {
		input.Tags = aws.StringMap(oidc.Tags)
	}

	_, err := a.plan.eksClient.AssociateIdentityProviderConfigWithContext(ctx, input)
	if err != nil {
		return errors.Wrap(err, "failed associating identity provider")
	}

	return nil
}

type UpdatedIdentityProviderTagsProcedure struct {
	plan *plan
}

func (u *UpdatedIdentityProviderTagsProcedure) Name() string {
	return "update_identity_provider_tags"
}

func (u *UpdatedIdentityProviderTagsProcedure) Do(ctx context.Context) error {
	arn := u.plan.currentIdentityProvider.IdentityProviderConfigArn
	_, err := u.plan.eksClient.TagResource(&eks.TagResourceInput{
		ResourceArn: arn,
		Tags:        aws.StringMap(u.plan.desiredIdentityProvider.Tags),
	})

	if err != nil {
		return errors.Wrap(err, "updating identity provider tags")
	}

	return nil
}

type RemoveIdentityProviderTagsProcedure struct {
	plan *plan
}

func (r *RemoveIdentityProviderTagsProcedure) Name() string {
	return "remove_identity_provider_tags"
}

func (r *RemoveIdentityProviderTagsProcedure) Do(ctx context.Context) error {
	keys := make([]*string, 0, len(r.plan.currentIdentityProvider.Tags))

	for key := range r.plan.currentIdentityProvider.Tags {
		keys = append(keys, aws.String(key))
	}
	_, err := r.plan.eksClient.UntagResource(&eks.UntagResourceInput{
		ResourceArn: r.plan.currentIdentityProvider.IdentityProviderConfigArn,
		TagKeys:     keys,
	})

	if err != nil {
		return errors.Wrap(err, "untagging identity provider")
	}
	return nil
}
