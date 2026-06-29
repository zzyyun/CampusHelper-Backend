const api=require('../../utils/api')
const {TASK_TYPE_TEXT,TASK_TYPE_TAG,TASK_STATUS_TEXT,PAGE_SIZE}=require('../../utils/constants')
Page({
  data:{
    tabs:[{id:0,name:'全部'},{id:1,name:'跑腿代拿'},{id:2,name:'组队/拼车'},{id:3,name:'悬赏求助'}],
    activeTab:0,tasks:[],cursor:'',hasMore:true,loading:false,
    taskTypeTag:TASK_TYPE_TAG,taskTypeText:TASK_TYPE_TEXT,taskStatusText:TASK_STATUS_TEXT,
    
  },
  onShow(){if(this.data.tasks.length===0)this.loadTasks()},
  onPullDownRefresh(){this.data.cursor='';this.data.hasMore=true;this.loadTasks(true).finally(()=>wx.stopPullDownRefresh())},
  onReachBottom(){if(this.data.hasMore&&!this.data.loading)this.loadTasks()},
  loadTasks(r){if(this.data.loading)return;this.setData({loading:true})
    api.listTasks({cursor:r?'':this.data.cursor,taskType:this.data.activeTab}).then(d=>{this.setData({tasks:r?(d.tasks||[]):this.data.tasks.concat(d.tasks||[]),cursor:d.next_cursor||'',hasMore:d.has_more,loading:false})}).catch(()=>this.setData({loading:false}))},
  onTabChange(e){const t=e.currentTarget.dataset.tab;if(t===this.data.activeTab)return;this.setData({activeTab:t,tasks:[],cursor:'',hasMore:true});this.loadTasks()},
  onTaskTap(e){wx.navigateTo({url:'/pages/task/detail?id='+e.currentTarget.dataset.id})},
  onCreateTask(){wx.navigateTo({url:'/pages/task/create'})}
})