name := "mugi"
format_command := "~/go/bin/gofumpt -l -w ."

default:
  just --list

fmt:
  {{ format_command }}

run *arguments="":
  go run ./cmd/{{ name }} {{ arguments }}

build:
  go build -o {{ name }} ./cmd/{{ name }}

install path="~/.local/bin" *arguments="":
  install -Dm755 {{ name }} {{ path }}/{{ name }}

clean:
  rm -rf {{ name }}
