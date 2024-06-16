package aws

import (
    "github.com/alustan/terraform-controller/plugin"
)

type AWSProvider struct{}

func init() {
    plugin.RegisterProvider("aws", &AWSProvider{})
}

func (p *AWSProvider) GetDockerfileAdditions() string {
    return `RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
            apt install unzip && \
            unzip awscliv2.zip && \
            ./aws/install && \
            rm -rf awscliv2.zip aws`
}

