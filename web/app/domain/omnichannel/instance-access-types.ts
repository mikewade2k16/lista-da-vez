export interface OmniInstanceCapabilities {
  view: boolean
  reply: boolean
  manage: boolean
  resetHistory: boolean
}

export type OmniInstanceAccessPolicy = 'ACCOUNT_SHARED' | 'RESTRICTED'
export type OmniInstanceGrantLevel = 'view' | 'reply' | 'manage'

export interface OmniInstanceGrant {
  userId: string
  accessLevel: OmniInstanceGrantLevel
  isActive: boolean
  revision: number
}

export interface OmniInstanceAccessAdmin {
  accessRevision: number
  accessPolicy: OmniInstanceAccessPolicy
  responsibleUserId: string | null
  grants: OmniInstanceGrant[]
  myCapabilities: OmniInstanceCapabilities
}

export interface OmniInstanceAccessWrite {
  accessRevision: number
  accessPolicy: OmniInstanceAccessPolicy
  responsibleUserId: string
  grants: Array<Pick<OmniInstanceGrant, 'userId' | 'accessLevel'>>
}
