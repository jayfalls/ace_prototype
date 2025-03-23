[⬆️](..)

## Component Commands
- UI
```shell
npm10 run start-local
```
- Controller
```shell
source ../.venv/bin/activate
ACE_LOGGER_FILE_NAME="controller" ACE_LOGGER_VERBOSE="" ./component.py --controller --dev
```

## Podman Weirdness
```shell
podman machine stop && podman machine start
```