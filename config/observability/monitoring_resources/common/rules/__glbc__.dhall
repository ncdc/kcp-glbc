let rules =
      [ ./CertManagerDown.dhall
      , ./GLBCTargetDown.dhall
      , ./HighDNSLatencyAlert.dhall
      , ./HighDNSProviderErrorRate.dhall
      , ./HighTLSProviderErrorRate.dhall
      , ./HighTLSProviderLatencyAlert.dhall
      ]

in  rules
