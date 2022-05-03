terraform {
  required_version = ">= 0.14.0"
  required_providers {
    confluentcloud = {
      source  = "confluentinc/confluentcloud"
      version = "0.6.0"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 2.55.0"
    }
  }
}

provider "confluentcloud" {
  api_key    = var.confluent_cloud_api_key
  api_secret = var.confluent_cloud_api_secret
}

resource "confluentcloud_environment" "staging" {
  display_name = "Staging"
}

resource "confluentcloud_network" "private-link" {
  display_name     = "Private Link Network"
  cloud            = "AZURE"
  region           = var.region
  connection_types = ["PRIVATELINK"]
  environment {
    id = confluentcloud_environment.staging.id
  }

  lifecycle { ignore_changes = [display_name, ] }
}

resource "confluentcloud_private_link_access" "azure" {
  display_name = "Azure Private Link Access"
  azure {
    subscription = var.subscription_id
  }
  environment {
    id = confluentcloud_environment.staging.id
  }
  network {
    id = confluentcloud_network.private-link.id
  }
}

resource "confluentcloud_kafka_cluster" "dedicated" {
  display_name = "inventory"
  availability = "MULTI_ZONE"
  cloud        = confluentcloud_network.private-link.cloud
  region       = confluentcloud_network.private-link.region
  dedicated {
    cku = 2
  }
  environment {
    id = confluentcloud_environment.staging.id
  }
  network {
    id = confluentcloud_network.private-link.id
  }
}

// 'app-manager' service account is required in this configuration to create 'orders' topic and assign roles
// to 'app-producer' and 'app-consumer' service accounts.
resource "confluentcloud_service_account" "app-manager" {
  display_name = "app-manager"
  description  = "Service account to manage 'inventory' Kafka cluster"
}

resource "confluentcloud_role_binding" "app-manager-kafka-cluster-admin" {
  principal   = "User:${confluentcloud_service_account.app-manager.id}"
  role_name   = "CloudClusterAdmin"
  crn_pattern = confluentcloud_kafka_cluster.dedicated.rbac_crn
}

resource "confluentcloud_api_key" "app-manager-kafka-api-key" {
  display_name = "app-manager-kafka-api-key"
  description  = "Kafka API Key that is owned by 'app-manager' service account"
  owner {
    id          = confluentcloud_service_account.app-manager.id
    api_version = confluentcloud_service_account.app-manager.api_version
    kind        = confluentcloud_service_account.app-manager.kind
  }

  managed_resource {
    id          = confluentcloud_kafka_cluster.dedicated.id
    api_version = confluentcloud_kafka_cluster.dedicated.api_version
    kind        = confluentcloud_kafka_cluster.dedicated.kind

    environment {
      id = confluentcloud_environment.staging.id
    }
  }

  # The goal is to ensure that
  # 1. confluentcloud_role_binding.app-manager-kafka-cluster-admin is created before
  # confluentcloud_api_key.app-manager-kafka-api-key is used to create instances of
  # confluentcloud_kafka_topic resource.
  # 2. Kafka connectivity through Azure PrivateLink is setup.
  depends_on = [
    confluentcloud_role_binding.app-manager-kafka-cluster-admin,

    confluentcloud_private_link_access.azure,
    azurerm_private_dns_zone_virtual_network_link.hz,
    azurerm_private_dns_a_record.rr,
    azurerm_private_dns_a_record.zonal
  ]
}

resource "confluentcloud_kafka_topic" "orders" {
  kafka_cluster {
    id = confluentcloud_kafka_cluster.dedicated.id
  }
  topic_name    = "orders"
  http_endpoint = confluentcloud_kafka_cluster.dedicated.http_endpoint
  credentials {
    key    = confluentcloud_api_key.app-manager-kafka-api-key.id
    secret = confluentcloud_api_key.app-manager-kafka-api-key.secret
  }
}

resource "confluentcloud_service_account" "app-consumer" {
  display_name = "app-consumer"
  description  = "Service account to consume from 'orders' topic of 'inventory' Kafka cluster"
}

resource "confluentcloud_api_key" "app-consumer-kafka-api-key" {
  display_name = "app-consumer-kafka-api-key"
  description  = "Kafka API Key that is owned by 'app-consumer' service account"
  owner {
    id          = confluentcloud_service_account.app-consumer.id
    api_version = confluentcloud_service_account.app-consumer.api_version
    kind        = confluentcloud_service_account.app-consumer.kind
  }

  managed_resource {
    id          = confluentcloud_kafka_cluster.dedicated.id
    api_version = confluentcloud_kafka_cluster.dedicated.api_version
    kind        = confluentcloud_kafka_cluster.dedicated.kind

    environment {
      id = confluentcloud_environment.staging.id
    }
  }

  depends_on = [
    confluentcloud_private_link_access.azure,
    azurerm_private_dns_zone_virtual_network_link.hz,
    azurerm_private_dns_a_record.rr,
    azurerm_private_dns_a_record.zonal
  ]
}

resource "confluentcloud_role_binding" "app-producer-developer-write" {
  principal   = "User:${confluentcloud_service_account.app-producer.id}"
  role_name   = "DeveloperWrite"
  crn_pattern = "${confluentcloud_kafka_cluster.dedicated.rbac_crn}/kafka=${confluentcloud_kafka_cluster.dedicated.id}/topic=${confluentcloud_kafka_topic.orders.topic_name}"
}

resource "confluentcloud_service_account" "app-producer" {
  display_name = "app-producer"
  description  = "Service account to produce to 'orders' topic of 'inventory' Kafka cluster"
}

resource "confluentcloud_api_key" "app-producer-kafka-api-key" {
  display_name = "app-producer-kafka-api-key"
  description  = "Kafka API Key that is owned by 'app-producer' service account"
  owner {
    id          = confluentcloud_service_account.app-producer.id
    api_version = confluentcloud_service_account.app-producer.api_version
    kind        = confluentcloud_service_account.app-producer.kind
  }

  managed_resource {
    id          = confluentcloud_kafka_cluster.dedicated.id
    api_version = confluentcloud_kafka_cluster.dedicated.api_version
    kind        = confluentcloud_kafka_cluster.dedicated.kind

    environment {
      id = confluentcloud_environment.staging.id
    }
  }

  depends_on = [
    confluentcloud_private_link_access.azure,
    azurerm_private_dns_zone_virtual_network_link.hz,
    azurerm_private_dns_a_record.rr,
    azurerm_private_dns_a_record.zonal
  ]
}

// Note that in order to consume from a topic, the principal of the consumer ('app-consumer' service account)
// needs to be authorized to perform 'READ' operation on both Topic and Group resources:
resource "confluentcloud_role_binding" "app-producer-developer-read-from-topic" {
  principal   = "User:${confluentcloud_service_account.app-consumer.id}"
  role_name   = "DeveloperRead"
  crn_pattern = "${confluentcloud_kafka_cluster.dedicated.rbac_crn}/kafka=${confluentcloud_kafka_cluster.dedicated.id}/topic=${confluentcloud_kafka_topic.orders.topic_name}"
}

resource "confluentcloud_role_binding" "app-producer-developer-read-from-group" {
  principal = "User:${confluentcloud_service_account.app-consumer.id}"
  role_name = "DeveloperRead"
  // The existing value of crn_pattern's suffix (group=confluent_cli_consumer_*) are set up to match Confluent CLI's default consumer group ID ("confluent_cli_consumer_<uuid>").
  // https://docs.confluent.io/confluent-cli/current/command-reference/kafka/topic/confluent_kafka_topic_consume.html
  // Update it to match your target consumer group ID.
  crn_pattern = "${confluentcloud_kafka_cluster.dedicated.rbac_crn}/kafka=${confluentcloud_kafka_cluster.dedicated.id}/group=confluent_cli_consumer_*"
}

# https://docs.confluent.io/cloud/current/networking/private-links/azure-privatelink.html
# Set up Private Endpoints for Azure Private Link in your Azure subscription
# Set up DNS records to use Azure Private Endpoints
provider "azurerm" {
  features {
  }
  subscription_id = var.subscription_id
  client_id       = var.client_id
  client_secret   = var.client_secret
  tenant_id       = var.tenant_id
}

locals {
  hosted_zone = replace(regex("^[^.]+-([0-9a-zA-Z]+[.].*):[0-9]+$", confluentcloud_kafka_cluster.dedicated.http_endpoint)[0], "glb.", "")
  network_id  = regex("^([^.]+)[.].*", local.hosted_zone)[0]
}

data "azurerm_resource_group" "rg" {
  name = var.resource_group
}

data "azurerm_virtual_network" "vnet" {
  name                = var.vnet_name
  resource_group_name = data.azurerm_resource_group.rg.name
}

data "azurerm_subnet" "subnet" {
  for_each = var.subnet_name_by_zone

  name                 = each.value
  virtual_network_name = data.azurerm_virtual_network.vnet.name
  resource_group_name  = data.azurerm_resource_group.rg.name
}

locals {
  assert_no_network_policies_enabled = length([
    for _, subnet in data.azurerm_subnet.subnet :
    true if ! subnet.enforce_private_link_endpoint_network_policies
  ]) > 0 ? file("\n\nerror: private link endpoint network policies must be disabled https://docs.microsoft.com/en-us/azure/private-link/disable-private-endpoint-network-policy") : ""
}

resource "azurerm_private_dns_zone" "hz" {
  resource_group_name = data.azurerm_resource_group.rg.name

  name = local.hosted_zone
}

resource "azurerm_private_endpoint" "endpoint" {
  for_each = confluentcloud_network.private-link.azure[0].private_link_service_aliases

  name                = "confluent-${local.network_id}-${each.key}"
  location            = var.region
  resource_group_name = data.azurerm_resource_group.rg.name

  subnet_id = data.azurerm_subnet.subnet[each.key].id

  private_service_connection {
    name                              = "confluent-${local.network_id}-${each.key}"
    is_manual_connection              = true
    private_connection_resource_alias = each.value
    request_message                   = "PL"
  }
}

resource "azurerm_private_dns_zone_virtual_network_link" "hz" {
  name                  = data.azurerm_virtual_network.vnet.name
  resource_group_name   = data.azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.hz.name
  virtual_network_id    = data.azurerm_virtual_network.vnet.id
}

resource "azurerm_private_dns_a_record" "rr" {
  name                = "*"
  zone_name           = azurerm_private_dns_zone.hz.name
  resource_group_name = data.azurerm_resource_group.rg.name
  ttl                 = 60
  records = [
    for _, ep in azurerm_private_endpoint.endpoint : ep.private_service_connection[0].private_ip_address
  ]
}

resource "azurerm_private_dns_a_record" "zonal" {
  for_each = confluentcloud_network.private-link.azure[0].private_link_service_aliases

  name                = "*.az${each.key}"
  zone_name           = azurerm_private_dns_zone.hz.name
  resource_group_name = data.azurerm_resource_group.rg.name
  ttl                 = 60
  records = [
    azurerm_private_endpoint.endpoint[each.key].private_service_connection[0].private_ip_address,
  ]
}