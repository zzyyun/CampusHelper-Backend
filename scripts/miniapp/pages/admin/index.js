const api=require('../../utils/api')
Page({
  data:{},
  onShow(){wx.setNavigationBarTitle({title:'管理后台'})},
  onNavigate(e){const url=e.currentTarget.dataset.url;if(url)wx.navigateTo({url})}
})
