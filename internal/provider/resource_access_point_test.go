// Copyright 2024 Confluent Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/walkerus/go-wiremock"
)

const (
	scenarioStateAccessPointIsProvisioning = "The new access point is provisioning"
	scenarioStateAccessPointHasBeenCreated = "The new access point has been just created"
	scenarioStateAccessPointHasBeenUpdated = "The new access point has been updated"
	awsEgressAccessPointScenarioName       = "confluent_access_point Aws Egress Private Link Endpoint Resource Lifecycle"
	azureEgressAccessPointScenarioName     = "confluent_access_point Azure Egress Private Link Endpoint Resource Lifecycle"

	accessPointUrlPath       = "/networking/v1/access-points"
	accessPointResourceLabel = "confluent_access_point.main"
)

func TestAccAccessPointAwsEgressPrivateLinkEndpoint(t *testing.T) {
	ctx := context.Background()

	wiremockContainer, err := setupWiremock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer wiremockContainer.Terminate(ctx)

	mockServerUrl := wiremockContainer.URI
	wiremockClient := wiremock.NewClient(mockServerUrl)
	// nolint:errcheck
	defer wiremockClient.Reset()

	// nolint:errcheck
	defer wiremockClient.ResetAllScenarios()

	createAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/create_aws_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Post(wiremock.URLPathEqualTo(accessPointUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WhenScenarioStateIs(wiremock.ScenarioStateStarted).
		WillSetStateTo(scenarioStateAccessPointIsProvisioning).
		WillReturn(
			string(createAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusCreated,
		))

	accessPointReadUrlPath := fmt.Sprintf("%s/ap-abc123", accessPointUrlPath)

	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointIsProvisioning).
		WillSetStateTo(scenarioStateAccessPointHasBeenCreated).
		WillReturn(
			string(createAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	readCreatedAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/read_created_aws_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenCreated).
		WillReturn(
			string(readCreatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	updatedAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/update_aws_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Patch(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenCreated).
		WillSetStateTo(scenarioStateAccessPointHasBeenUpdated).
		WillReturn(
			string(updatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenUpdated).
		WillReturn(
			string(updatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	_ = wiremockClient.StubFor(wiremock.Delete(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(awsEgressAccessPointScenarioName).
		WillReturn(
			"",
			contentTypeJSONHeader,
			http.StatusNoContent,
		))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		// https://www.terraform.io/docs/extend/testing/acceptance-tests/teststep.html
		// https://www.terraform.io/docs/extend/best-practices/testing.html#built-in-patterns
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceAccessPointAwsEgressWithIdSet(mockServerUrl),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(accessPointResourceLabel, "id", "ap-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "display_name", "prod-ap-1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.0.id", "env-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.0.id", "gw-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.#", "0"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_service_name", "com.amazonaws.vpce.us-west-2.vpce-svc-00000000000000000"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_id", "vpce-00000000000000000"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_dns_name", "*.vpce-00000000000000000-abcd1234.s3.us-west-2.vpce.amazonaws.com"),
				),
			},
			{
				Config: testAccCheckResourceUpdateAccessPointAwsEgressWithIdSet(mockServerUrl),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(accessPointResourceLabel, "id", "ap-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "display_name", "prod-ap-2"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.0.id", "env-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.0.id", "gw-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.#", "0"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_service_name", "com.amazonaws.vpce.us-west-2.vpce-svc-00000000000000000"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_id", "vpce-00000000000000000"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.0.vpc_endpoint_dns_name", "*.vpce-00000000000000000-abcd1234.s3.us-west-2.vpce.amazonaws.com"),
				),
			},
		},
	})
}

func TestAccAccessPointAzureEgressPrivateLinkEndpoint(t *testing.T) {
	ctx := context.Background()

	wiremockContainer, err := setupWiremock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer wiremockContainer.Terminate(ctx)

	mockServerUrl := wiremockContainer.URI
	wiremockClient := wiremock.NewClient(mockServerUrl)
	// nolint:errcheck
	defer wiremockClient.Reset()

	// nolint:errcheck
	defer wiremockClient.ResetAllScenarios()

	createAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/create_azure_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Post(wiremock.URLPathEqualTo(accessPointUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WhenScenarioStateIs(wiremock.ScenarioStateStarted).
		WillSetStateTo(scenarioStateAccessPointIsProvisioning).
		WillReturn(
			string(createAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusCreated,
		))

	accessPointReadUrlPath := fmt.Sprintf("%s/ap-def456", accessPointUrlPath)

	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointIsProvisioning).
		WillSetStateTo(scenarioStateAccessPointHasBeenCreated).
		WillReturn(
			string(createAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	readCreatedAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/read_created_azure_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenCreated).
		WillReturn(
			string(readCreatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	updatedAccessPointResponse, _ := os.ReadFile("../testdata/network_access_point/update_azure_egress_ap.json")
	_ = wiremockClient.StubFor(wiremock.Patch(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenCreated).
		WillSetStateTo(scenarioStateAccessPointHasBeenUpdated).
		WillReturn(
			string(updatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	_ = wiremockClient.StubFor(wiremock.Get(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WhenScenarioStateIs(scenarioStateAccessPointHasBeenUpdated).
		WillReturn(
			string(updatedAccessPointResponse),
			contentTypeJSONHeader,
			http.StatusOK,
		))

	_ = wiremockClient.StubFor(wiremock.Delete(wiremock.URLPathEqualTo(accessPointReadUrlPath)).
		InScenario(azureEgressAccessPointScenarioName).
		WillReturn(
			"",
			contentTypeJSONHeader,
			http.StatusNoContent,
		))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		// https://www.terraform.io/docs/extend/testing/acceptance-tests/teststep.html
		// https://www.terraform.io/docs/extend/best-practices/testing.html#built-in-patterns
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceAccessPointAzureEgressWithIdSet(mockServerUrl),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(accessPointResourceLabel, "id", "ap-def456"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "display_name", "prod-ap-1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.0.id", "env-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.0.id", "gw-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.#", "0"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_link_service_resource_id", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/s-abcde/providers/Microsoft.Network/privateLinkServices/pls-plt-abcdef-az3"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_link_subresource_name", "sqlServer"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_resource_id", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testvpc/providers/Microsoft.Network/privateEndpoints/pe-plt-abcdef-az3"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_domain", "dbname.database.windows.net"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_ip_address", "10.2.0.68"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.#", "2"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.0", "dbname.database.windows.net"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.1", "dbname-region.database.windows.net"),
				),
			},
			{
				Config: testAccCheckResourceUpdateAccessPointAzureEgressWithIdSet(mockServerUrl),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(accessPointResourceLabel, "id", "ap-def456"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "display_name", "prod-ap-2"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "environment.0.id", "env-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "gateway.0.id", "gw-abc123"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.#", "1"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "aws_egress_private_link_endpoint.#", "0"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_link_service_resource_id", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/s-abcde/providers/Microsoft.Network/privateLinkServices/pls-plt-abcdef-az3"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_link_subresource_name", "sqlServer"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_resource_id", "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testvpc/providers/Microsoft.Network/privateEndpoints/pe-plt-abcdef-az3"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_domain", "dbname.database.windows.net"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_ip_address", "10.2.0.68"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.#", "2"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.0", "dbname.database.windows.net"),
					resource.TestCheckResourceAttr(accessPointResourceLabel, "azure_egress_private_link_endpoint.0.private_endpoint_custom_dns_config_domains.1", "dbname-region.database.windows.net"),
				),
			},
		},
	})
}

func testAccCheckResourceAccessPointAwsEgressWithIdSet(mockServerUrl string) string {
	return fmt.Sprintf(`
    provider "confluent" {
        endpoint = "%s"
    }

	resource "confluent_access_point" "main" {
		display_name = "prod-ap-1"
		environment {
			id = "env-abc123"
		}
		gateway {
			id = "gw-abc123"
		}
		aws_egress_private_link_endpoint {
			vpc_endpoint_service_name = "com.amazonaws.vpce.us-west-2.vpce-svc-00000000000000000"
		}
	}
	`, mockServerUrl)
}

func testAccCheckResourceUpdateAccessPointAwsEgressWithIdSet(mockServerUrl string) string {
	return fmt.Sprintf(`
    provider "confluent" {
        endpoint = "%s"
    }

	resource "confluent_access_point" "main" {
		display_name = "prod-ap-2"
		environment {
			id = "env-abc123"
		}
		gateway {
			id = "gw-abc123"
		}
		aws_egress_private_link_endpoint {
			vpc_endpoint_service_name = "com.amazonaws.vpce.us-west-2.vpce-svc-00000000000000000"
		}
	}
	`, mockServerUrl)
}

func testAccCheckResourceAccessPointAzureEgressWithIdSet(mockServerUrl string) string {
	return fmt.Sprintf(`
    provider "confluent" {
        endpoint = "%s"
    }

	resource "confluent_access_point" "main" {
		display_name = "prod-ap-1"
		environment {
			id = "env-abc123"
		}
		gateway {
			id = "gw-abc123"
		}
		azure_egress_private_link_endpoint {
			private_link_service_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/s-abcde/providers/Microsoft.Network/privateLinkServices/pls-plt-abcdef-az3"
			private_link_subresource_name = "sqlServer"
		}
	}
	`, mockServerUrl)
}

func testAccCheckResourceUpdateAccessPointAzureEgressWithIdSet(mockServerUrl string) string {
	return fmt.Sprintf(`
    provider "confluent" {
        endpoint = "%s"
    }

	resource "confluent_access_point" "main" {
		display_name = "prod-ap-2"
		environment {
			id = "env-abc123"
		}
		gateway {
			id = "gw-abc123"
		}
		azure_egress_private_link_endpoint {
			private_link_service_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/s-abcde/providers/Microsoft.Network/privateLinkServices/pls-plt-abcdef-az3"
			private_link_subresource_name = "sqlServer"
		}
	}
	`, mockServerUrl)
}
