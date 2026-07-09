const api=require('../../utils/api'),app=getApp()
Page({
  data:{userInfo:null,schoolName:'',unreadCount:0,avatarText:'?',isAdmin:false,isGuest:true},
  onShow(){
    const isGuest=app.globalData.isGuest||!wx.getStorageSync('accessToken')
    this.setData({isGuest})
    if(isGuest){
      this.setData({userInfo:null,schoolName:'',avatarText:'?',isAdmin:false,unreadCount:0})
      return
    }
    this.loadUserInfo()
    this.loadUnreadCount()
  },
  loadUserInfo(){api.getMyInfo().then(d=>{const nick=d.nickname||'我';app.globalData.userInfo=d;this.setData({userInfo:d,schoolName:d.school_name||app.globalData.schoolName||'',avatarText:nick[0]||'?',isAdmin:d.role&&d.role!=='student'})}).catch(()=>{})},
  loadUnreadCount(){api.getUnreadCount().then(d=>this.setData({unreadCount:d.count||0})).catch(()=>{})},
  onNavigate(e){
    const url=e.currentTarget.dataset.url
    if(!url)return
    // 游客点击需要登录的功能时引导登录
    if(this.data.isGuest&&url!=='/pages/search/search'){
      if(!api.requireLogin())return
    }
    wx.navigateTo({url})
  },
  onGoLogin(){wx.navigateTo({url:'/pages/login/login'})},
  onLogout(){wx.showModal({title:'确认退出',content:'确定要退出登录吗？',success:r=>{if(r.confirm){wx.clearStorageSync();app.globalData.accessToken='';app.globalData.userInfo=null;app.globalData.isBoundCampus=false;app.globalData.isGuest=true;wx.redirectTo({url:'/pages/login/login'})}}})}
})
