import { defineStore } from 'pinia'

export const useSessionStore = defineStore('session', {
  state: () => ({
    user: {
      name: 'orbit',
      trustLevel: '信任等级4',
      badges: ['已绑定 linux.do', '个人车主'],
    },
  }),
})
