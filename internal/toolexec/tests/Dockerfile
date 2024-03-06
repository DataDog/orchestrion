FROM golang:1
WORKDIR /orchestrion/internal/toolexec/tests
RUN apt update && apt install -y bash
ENTRYPOINT ["bash", "-c"]
CMD ./tests.sh
