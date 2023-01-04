[The general structure of the DSL is shown below (partial list)](https://pkg.go.dev/goa.design/goa/dsl#section-documentation)
```shell
API                 Service          Type            ResultType
├── Title           ├── Description  ├── Extend      ├── TypeName
├── Description     ├── Docs         ├── Reference   ├── ContentType
├── Version         ├── Security     ├── ConvertTo   ├── Extend
├── Docs            ├── Error        ├── CreateFrom  ├── Reference
├── License         ├── GRPC         ├── Attribute   ├── ConvertTo
├── TermsOfService  ├── HTTP         ├── Field       ├── CreateFrom
├── Contact         ├── Method       └── Required    ├── Attributes
├── Server          │   ├── Payload                  └── View
└── HTTP            │   ├── Result
                    │   ├── Error
                    │   ├── GRPC
                    │   └── HTTP
                    └── Files
```
