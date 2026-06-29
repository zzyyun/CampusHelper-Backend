const api=require('../../utils/api'),app=getApp()
Page({
  data:{userInfo:null,schoolName:'',unreadCount:0,avatarText:'?',isAdmin:false},
  onShow(){this.loadUserInfo();this.loadUnreadCount()},
    loadUserInfo(){api.getMyInfo().then(d=>{const nick=d.nickname||'我';app.globalData.userInfo=d;this.setData({userInfo:d,schoolName:d.school_name||app.globalData.schoolName||'',avatarText:nick[0]||'?',isAdmin:d.role&&d.role!=='student'})}).catch(()=>{})},
  loadUnreadCount(){api.getUnreadCount().then(d=>this.setData({unreadCount:d.count||0})).catch(()=>{})},
  onNavigate(e){const url=e.currentTarget.dataset.url;if(url)wx.navigateTo({url})},
  onLogout(){wx.showModal({title:'确认退出',content:'确定要退出登录吗？',success:r=>{if(r.confirm){wx.clearStorageSync();app.globalData.accessToken='';app.globalData.userInfo=null;app.globalData.isBoundCampus=false;wx.redirectTo({url:'/pages/login/login'})}}})}
})
