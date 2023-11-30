provider "aws" {
  region = "us-east-1"
}

data "aws_iam_policy_document" "assume_ecs_task" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_task" {
  name               = "ecs-task-execute-role"
  assume_role_policy = data.aws_iam_policy_document.assume_ecs_task.json
}

resource "aws_iam_role_policy_attachment" "ecs_task" {
  role       = aws_iam_role.ecs_task.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

data "aws_iam_policy_document" "assume_lambda" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "lambda-execute-role"
  assume_role_policy = data.aws_iam_policy_document.assume_lambda.json
}

data "aws_iam_policy_document" "sqs_message_manage" {
  statement {
    actions = [
      "sqs:SendMessage",
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
      "ecs:RunTask",
      "iam:PassRole"
    ]
    resources = [
      aws_sqs_queue.myshoes-queue.arn,
      aws_ecs_task_definition.myshoes.arn,
      aws_iam_role.ecs_task.arn
    ]
  }
}

resource "aws_iam_policy" "sqs_message_manage" {
  policy = data.aws_iam_policy_document.sqs_message_manage.json
}

resource "aws_iam_role_policy_attachment" "ecs_task_sqs_message_manage" {
  role       = aws_iam_role.lambda.name
  policy_arn = aws_iam_policy.sqs_message_manage.arn
}

// For myshoes-serverless-aws
resource "aws_lambda_function" "httpserver" {
  filename      = "../../dist/lambda/httpserver.zip"
  function_name = "myshoes_httpserver"
  role          = aws_iam_role.lambda.arn
  handler       = "httpserver"
  runtime       = "go1.x"

  source_code_hash = filebase64sha256("../../dist/lambda/httpserver.zip")

  environment {
    variables = {
      AWS_SQS_QUEUE_URL         = aws_sqs_queue.myshoes-queue.url
      DEBUG                     = true
      STRICT                    = false
      PLUGIN                    = "/tmp/shoes-ecs-task"
      GITHUB_APP_ID             = ""
      GITHUB_APP_SECRET         = ""
      GITHUB_PRIVATE_KEY_BASE64 = ""
      MODE_WEBHOOK_TYPE         = "workflow_job"
    }
  }
}

resource "aws_lambda_function_url" "httpserver" {
  function_name      = aws_lambda_function.httpserver.function_name
  authorization_type = "NONE"
}

resource "aws_cloudwatch_log_group" "httpserver" {
  name = "/aws/lambda/${aws_lambda_function.httpserver.function_name}"
}

resource "aws_lambda_function" "dispatcher" {
  filename      = "../../dist/lambda/dispatcher.zip"
  function_name = "myshoes_dispatcher"
  role          = aws_iam_role.lambda.arn
  handler       = "dispatcher"
  runtime       = "go1.x"

  source_code_hash = filebase64sha256("../../dist/lambda/dispatcher.zip")

  timeout = 30

  environment {
    variables = {
      AWS_SQS_QUEUE_URL         = aws_sqs_queue.myshoes-queue.url
      DEBUG                     = true
      STRICT                    = false
      PLUGIN                    = "/tmp/shoes-ecs-task"
      GITHUB_APP_ID             = ""
      GITHUB_APP_SECRET         = ""
      GITHUB_PRIVATE_KEY_BASE64 = ""
      MODE_WEBHOOK_TYPE         = "workflow_job"

      ECS_TASK_CLUSTER        = aws_ecs_cluster.myshoes.name
      ECS_TASK_DEFINITION_ARN = aws_ecs_task_definition.myshoes.arn
      ECS_TASK_SUBNET_ID      = ""
      ECS_TASK_REGION         = ""
      ECS_TASK_NO_WAIT        = "true"
    }
  }
}

resource "aws_cloudwatch_log_group" "dispatcher" {
  name = "/aws/lambda/${aws_lambda_function.dispatcher.function_name}"
}

resource "aws_lambda_event_source_mapping" "dispatcher" {
  event_source_arn = aws_sqs_queue.myshoes-queue.arn
  function_name    = aws_lambda_function.dispatcher.function_name
}

// For shoes-ecs-task
resource "aws_ecs_cluster" "myshoes" {
  name = "myshoes"
}

resource "aws_ecs_cluster_capacity_providers" "farget_spot" {
  cluster_name       = aws_ecs_cluster.myshoes.name
  capacity_providers = ["FARGATE_SPOT"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    base              = 0
  }

  depends_on = [aws_ecs_cluster.myshoes]
}

resource "aws_cloudwatch_log_group" "runner" {
  name              = "/ecs/myshoes-ecs/runner"
  retention_in_days = 30
}

resource "aws_ecs_task_definition" "myshoes" {
  family = "myshoes"

  requires_compatibilities = ["FARGATE"]
  execution_role_arn       = aws_iam_role.ecs_task.arn

  cpu    = "256"
  memory = "512"

  network_mode = "awsvpc"

  container_definitions = <<EOL
[
  {
    "name": "runner",
    "image": "myoung34/github-runner-base",
    "logConfiguration": {
      "logDriver": "awslogs",
      "options": {
        "awslogs-region": "",
        "awslogs-stream-prefix": "runner",
        "awslogs-group": "/ecs/myshoes-ecs/runner"
      }
    }
  }
]
EOL
}

resource "aws_sqs_queue" "myshoes-queue" {
  name                        = "myshoes-serverless.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
}
