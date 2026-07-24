export interface GRPCSchema {
  services: GRPCService[]
  source: 'reflection' | 'proto_file' | 'proto_directory'
}

export interface GRPCService {
  fullName: string
  methods: GRPCMethodInfo[]
}

export interface GRPCMethodInfo {
  name: string
  inputType: string
  outputType: string
  isServerStream: boolean
  isClientStream: boolean
  protoDefinition: string
  exampleJson: string
}

export interface GRPCConnectRequest {
  host: string
  useTls: boolean
  protoPath: string
}

export interface GRPCGenerateExampleRequest extends GRPCConnectRequest {
  service: string
  method: string
}
