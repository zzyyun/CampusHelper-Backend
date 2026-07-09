App({
  globalData: {
    accessToken: '',
    refreshToken: '',
    userInfo: null,
    schoolId: 0,
    schoolName: '',
    isBoundCampus: false,
    isGuest: true
  },
  onLaunch() {
    const token = wx.getStorageSync('accessToken')
    if (token) {
      this.globalData.accessToken = token
      this.globalData.refreshToken = wx.getStorageSync('refreshToken') || ''
      this.globalData.isBoundCampus = !!wx.getStorageSync('isBoundCampus')
      this.globalData.isGuest = false
    }
  }
})
