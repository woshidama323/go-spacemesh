FROM golang:1.21 as builder

# Install git for cloning the repository
RUN set -ex \
   && apt-get update --fix-missing \
   && apt-get install -qy --no-install-recommends \
   git \
   unzip sudo \
   ocl-icd-opencl-dev

# Clone the repository
WORKDIR /src
RUN git clone https://github.com/woshidama323/go-spacemesh.git .

# Continue with the rest of the build process
COPY Makefile* .
RUN make get-libs

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go mod download

# Here we copy the rest of the source code
COPY . .

# And compile the project
RUN --mount=type=cache,id=build,target=/root/.cache/go-build make build
RUN --mount=type=cache,id=build,target=/root/.cache/go-build make gen-p2p-identity

# In this last stage, we start from a fresh image, to reduce the image size and not ship the Go compiler in our production artifacts.
FROM linux AS spacemesh

# Finally we copy the statically compiled Go binary.
COPY --from=builder /src/build/go-spacemesh /bin/
COPY --from=builder /src/build/service /bin/
COPY --from=builder /src/build/libpost.so /bin/
COPY --from=builder /src/build/gen-p2p-identity /bin/

ENTRYPOINT ["/bin/go-spacemesh"]
EXPOSE 7513

# profiling port
EXPOSE 6060

# pubsub port
EXPOSE 56565
