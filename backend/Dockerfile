# Build a static binary; web/friend.html is baked in via go:embed.
FROM golang:1.23 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# -p=1 keeps peak compiler memory low so this builds on small CI / Colima VMs.
RUN CGO_ENABLED=0 GOOS=linux go build -p=1 -o /pisah .

FROM gcr.io/distroless/static-debian12
COPY --from=build /pisah /pisah
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/pisah"]
