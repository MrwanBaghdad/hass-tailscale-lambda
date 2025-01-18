FROM golang:1.23-alpine as build
WORKDIR /build


# Run on container startup.
ENTRYPOINT ["/var/runtime/bootstrap"]

# Copy dependencies list
COPY go.mod go.sum ./
# Build with optional lambda.norpc tag
COPY main.go .
RUN go build -tags lambda.norpc -o main main.go

# Copy artifacts to a clean image
FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /build/main ./main

ENTRYPOINT [ "./main" ]

