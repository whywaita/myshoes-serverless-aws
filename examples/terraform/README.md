# Terraform example

## Prepare

Please update `main.tf` for your environment.

- `provider "aws"`: update region
- Environment values in lambda
  - GitHub Apps (see [docs in whywaita/myshoes](https://github.com/whywaita/myshoes/blob/master/docs/01_01_for_admin_setup.md))
    - GITHUB_APP_ID
    - GITHUB_APP_SECRET
    - GITHUB_PRIVATE_KEY_BASE64
  - AWS Environments
    - ECS_TASK_SUBNET_ID
    - ECS_TASK_REGION
- aws_ecs_task_definition

## Setup

```bash
# Build httpserver and dispatcher
$ make all

# Prepare terraform
$ cd examples/terraform
$ terraform init

# Create resources
$ terraform apply
```

Please set Lambda Function URL to GitHub Apps Webhook URL.

## Optional

- Create custom domain for Lambda Function URL
    - Lambda Function URL always change when you update Lambda Function. So you need to update GitHub Apps Webhook URL.
    - If you use custom domain, you don't need to update GitHub Apps Webhook URL.