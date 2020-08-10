# NotifyAwsBillingToSlack

This is a sample template for NotifyAwsBillingToSlack - Below is a brief explanation of what we have generated for you:

```bash
.
├── Makefile                    <-- Make to automate build
├── README.md                   <-- This instructions file
├── hello-world                 <-- Source code for a lambda function
│   └── main.go                 <-- Lambda function code
└── template.yaml
```

## Requirements

* AWS CLI already configured with Administrator permission
* [Golang](https://golang.org)
* SAM CLI - [Install the SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html)


## Packaging and deployment

To deploy your application for the first time, run the following in your shell:

```bash
$ git clone git@github.com:d-shimizu/NotifyAwsBillingToSlack.git

$ cd NotifyAwsBillingToSlack

$ sam build

$ sam deploy --guided

$ aws ssm put-parameter --name NotifyAwsBillingToSlack.WebHookUrl --value 'https://hooks.slack.com/services/********/********/************************' --type SecureString
```

## License

MIT

## Author

* [d-shimizu](https://github.com/d-shimizu)

