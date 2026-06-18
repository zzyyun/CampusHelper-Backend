Page({
  data: { code: '' },
  onLoad() {
    wx.login({
      success: (res) => this.setData({ code: res.code }),
    })
  },
  copy() {
    wx.setClipboardData({ data: this.data.code })
  },
})
