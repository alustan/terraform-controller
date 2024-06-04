# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY bin/terraform-controller  .
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/terraform-controller"]