const api=require('../../utils/api')
const {POST_TYPE_TEXT,POST_TYPE_TAG}=require('../../utils/constants')
Page({
  data:{
    tabs:[{id:0,name:'全部'},{id:1,name:'通用'},{id:2,name:'失物招领'},{id:3,name:'二手交易'}],
    activeTab:0,posts:[],cursor:'',hasMore:true,loading:false,
    schoolName:'',authorInitials:{},
    postTypeTag:POST_TYPE_TAG,postTypeText:POST_TYPE_TEXT
  },
  onShow(){
    const app=getApp()
    if(app.globalData.schoolName)this.setData({schoolName:app.globalData.schoolName})
    if(this.data.posts.length===0)this.loadPosts()
  },
  onPullDownRefresh(){this.data.cursor='';this.data.hasMore=true;this.loadPosts(true).finally(()=>wx.stopPullDownRefresh())},
  onReachBottom(){if(this.data.hasMore&&!this.data.loading)this.loadPosts()},
  loadPosts(r){
    if(this.data.loading)return;this.setData({loading:true})
    return api.listPosts({cursor:r?'':this.data.cursor,type:this.data.activeTab}).then(d=>{
      const posts=r?(d.posts||[]):this.data.posts.concat(d.posts||[])
      const initials={};posts.forEach(p=>{initials[p.id]=(p.author_name||'?')[0]})
      this.setData({posts,cursor:d.next_cursor||'',hasMore:d.has_more,loading:false,authorInitials:initials})
    }).catch(()=>this.setData({loading:false}))
  },
  onTabChange(e){const t=e.currentTarget.dataset.tab;if(t===this.data.activeTab)return;this.setData({activeTab:t,posts:[],cursor:'',hasMore:true});this.loadPosts()},
  onPostTap(e){wx.navigateTo({url:'/pages/post/detail?id='+e.currentTarget.dataset.id})},
  onSearch(){wx.navigateTo({url:'/pages/search/search'})},
  onCreatePost(){wx.navigateTo({url:'/pages/post/create'})}
})
