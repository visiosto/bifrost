# Bifröst

Bifröst (`bifrost`) is Visiosto’s server application for executing actions on
request from static websites. For example, a static website may need a back end
for a contact form. In many situations, a serverless function might suffice but,
to streamline the process and to allow additional and important functionality, a
simple HTTP server application like Bifröst might be a better and simpler fit.

Bifröst will be extended to support the common use cases that arise. Right now,
it can have endpoints that receive JSON payloads and send emails using Amazon
SES based on the payload, effectively acting as a backend for contact forms.

## Getting Started

The project requires POSIX `make` and Go 1.25 or newer. To build the project,
run

    make build

This creates `bifrost` executable at the project root.

## License

Copyright (c) 2025 Visiosto oy

This project is licensed under the Apache-2.0 license. See the
[LICENSE](LICENSE) file for more information.
