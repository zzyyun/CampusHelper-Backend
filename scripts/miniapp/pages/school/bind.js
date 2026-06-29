const api=require('../../utils/api')
const app=getApp()
Page({
  data:{keyword:'',schools:[],hasMore:false,cursor:'',loading:false},
  onLoad(){this.loadSchools()},
  onInput(e){this.data.keyword=e.detail.value;this.data.cursor='';this.loadSchools(true)},
  loadSchools(r){
    if(this.data.loading)return;this.setData({loading:true})
    api.listSchools(this.data.keyword,this.data.cursor).then(d=>{this.setData({schools:r?(d.schools||[]):this.data.schools.concat(d.schools||[]),hasMore:d.has_more,cursor:d.next_cursor||'',loading:false})}).catch(()=>this.setData({loading:false}))},
  onSelect(e){
    const school=e.currentTarget.dataset.school;if(!school)return
    wx.showLoading({title:'绑定中...',mask:true})
    api.bindCampus(school.name).then(()=>{
      app.globalData.isBoundCampus=true;app.globalData.schoolName=school.name;app.globalData.schoolId=school.school_id
      wx.hideLoading();wx.showToast({title:'绑定成功',icon:'success'});wx.switchTab({url:'/pages/home/home'})
    }).catch(()=>{wx.hideLoading();wx.showToast({title:'绑定失败',icon:'none'})})
  },
  onScrollToLower(){if(this.data.hasMore&&!this.data.loading)this.loadSchools()}
})
