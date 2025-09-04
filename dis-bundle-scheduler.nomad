job "dis-bundle-scheduler" {
  datacenters = ["eu-west-2"]
  region      = "eu"
  type        = "batch"

  periodic {
    cron             = "* * * * *"
    time_zone        = "UTC"
    prohibit_overlap = false
  }

  group "publishing" {
    count = "{{PUBLISHING_TASK_COUNT}}"

    constraint {
      attribute = "${node.class}"
      value     = "publishing-mount"
    }

    restart {
      attempts = 3
      delay    = "15s"
      interval = "1m"
      mode     = "delay"
    }

    task "dis-bundle-scheduler" {
      driver = "docker"

      artifact {
        source = "s3::https://s3-eu-west-2.amazonaws.com/{{DEPLOYMENT_BUCKET}}/dis-bundle-scheduler/{{PROFILE}}/{{RELEASE}}.tar.gz"
      }

      config {
        command = "${NOMAD_TASK_DIR}/start-task"

        args = ["./dis-bundle-scheduler"]

        image = "{{ECR_URL}}:concourse-{{REVISION}}"
      }

      resources {
        cpu    = "{{PUBLISHING_RESOURCE_CPU}}"
        memory = "{{PUBLISHING_RESOURCE_MEM}}"

        network {
          port "http" {}
        }
      }

      template {
        source      = "${NOMAD_TASK_DIR}/vars-template"
        destination = "${NOMAD_TASK_DIR}/vars"
      }

      vault {
        policies = ["dis-bundle-scheduler"]
      }
    }
  }
}
