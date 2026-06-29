Page({
  data:{loading:false,errorMsg:''},
  onLoad(){
    const token=wx.getStorageSync('accessToken')
    if(token){
      const app=getApp()
      app.globalData.accessToken=token
      app.globalData.refreshToken=wx.getStorageSync('refreshToken')||''
      const bound=wx.getStorageSync('isBoundCampus')
      app.globalData.isBoundCampus=!!bound
      if(bound){wx.switchTab({url:'/pages/home/home'})}
      else{wx.redirectTo({url:'/pages/school/bind'})}
    }
  },
  onGetUserProfile(){
    const that=this
    this.setData({errorMsg:'',loading:true})
    wx.login({
      success:(res)=>{
        if(!res.code){
          that.setData({loading:false,errorMsg:'获取登录临时码失败'})
          return
        }
        const api=require('../../utils/api')
        const app=getApp()
        api.wxLogin(res.code).then(data=>{
          wx.setStorageSync('accessToken',data.access_token)
          wx.setStorageSync('refreshToken',data.refresh_token)
          wx.setStorageSync('isBoundCampus',data.is_bound_campus)
          app.globalData.accessToken=data.access_token
          app.globalData.refreshToken=data.refresh_token
          app.globalData.isBoundCampus=data.is_bound_campus
          app.globalData.schoolId=data.school_id
          if(data.is_bound_campus){wx.switchTab({url:'/pages/home/home'})}
          else{wx.redirectTo({url:'/pages/school/bind'})}
        }).catch(e=>{
          that.setData({loading:false,errorMsg:'登录失败，请检查后端服务是否已启动'})
          console.error('login error:',e)
        })
      },
      fail:(err)=>{
        that.setData({loading:false,errorMsg:'微信登录异常，请重试'})
        console.error('wx.login error:',err)
      }
    })
  }
})
