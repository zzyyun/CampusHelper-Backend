const { BASE_URL, PAGE_SIZE } = require('./constants')

function getToken() { return wx.getStorageSync('accessToken') || '' }

function qs(obj) {
  return Object.entries(obj).filter(([_,v])=>v!==undefined&&v!==null&&v!=='').map(([k,v])=>encodeURIComponent(k)+'='+encodeURIComponent(v)).join('&')
}

function refreshToken() {
  return new Promise((resolve,reject)=>{
    const rt=wx.getStorageSync('refreshToken')
    if(!rt){reject(new Error('no refresh token'));return}
    wx.request({url:BASE_URL+'/user/refresh',method:'POST',data:{refresh_token:rt},success:res=>{
      if(res.data&&res.data.access_token){wx.setStorageSync('accessToken',res.data.access_token);resolve(res.data.access_token)}
      else{wx.clearStorageSync();wx.redirectTo({url:'/pages/login/login'});reject(new Error('refresh failed'))}
    },fail:reject})
  })
}

function request(method,path,data,options={}) {
  const {showLoading=false,retry=true}=options
  return new Promise((resolve,reject)=>{
    if(showLoading)wx.showLoading({title:'加载中...',mask:true})
    const header={'Content-Type':'application/json'};const token=getToken()
    if(token)header['Authorization']='Bearer '+token
    const url=path.startsWith('http')?path:BASE_URL+path
    wx.request({url,method,data,header,timeout:10000,
      success:res=>{
        if(showLoading)wx.hideLoading()
        if(res.data&&(res.data.code===20002||res.data.code===20003)&&retry){
          refreshToken().then(newToken=>request(method,path,data,{...options,retry:false}).then(resolve).catch(reject)).catch(reject)
          return
        }
        resolve(res.data||res)
      },
      fail:err=>{if(showLoading)wx.hideLoading();wx.showToast({title:'网络异常',icon:'none'});reject(err)}
    })
  })
}

// User
function wxLogin(code){return request('POST','/user/login',{code})}
function getMyInfo(){return request('GET','/user/me')}
function bindCampus(schoolName){return request('PUT','/user/campus',{school_name:schoolName})}
function updateUserInfo(data){return request('PUT','/user/info',data)}
function listSchools(keyword='',cursor=''){return request('GET','/schools?'+qs({keyword,cursor,page_size:PAGE_SIZE}))}

// Content
function listPosts({cursor='',pageSize=PAGE_SIZE,type=0,category=0,sort=0}={}){
  return request('GET','/content/posts?'+qs({cursor,page_size:pageSize,type,category,sort}))
}
function getPost(postId){return request('GET','/content/posts/'+postId)}
function createPost(data){return request('POST','/content/posts',data)}
function updatePost(postId,data){return request('PUT','/content/posts/'+postId,data)}
function deletePost(postId){return request('DELETE','/content/posts/'+postId)}
function likePost(postId){return request('POST','/content/posts/'+postId+'/like')}
function unlikePost(postId){return request('DELETE','/content/posts/'+postId+'/like')}
function searchContent({keyword,type=0,category=0,page=1,pageSize=PAGE_SIZE,sort=0}={}){
  return request('POST','/content/search',{keyword,type,category,page,page_size:pageSize,sort})
}
function listComments(postId,cursor='',pageSize=PAGE_SIZE){
  return request('GET','/content/posts/'+postId+'/comments?'+qs({cursor,page_size:pageSize}))
}
function listReplies(commentId,cursor='',pageSize=PAGE_SIZE){
  return request('GET','/content/comments/'+commentId+'/replies?'+qs({cursor,page_size:pageSize}))
}
function createComment(data){return request('POST','/content/comments',data)}
function deleteComment(commentId){return request('DELETE','/content/comments/'+commentId)}

// Tasks
function listTasks({cursor='',pageSize=PAGE_SIZE,taskType=0}={}){
  return request('GET','/tasks?'+qs({cursor,page_size:pageSize,task_type:taskType}))
}
function getTask(taskId){return request('GET','/tasks/'+taskId)}
function createTask(data){return request('POST','/tasks',data)}
function updateTask(taskId,data){return request('PUT','/tasks/'+taskId,data)}
function deleteTask(taskId){return request('DELETE','/tasks/'+taskId)}
function claimTask(taskId,data){return request('POST','/tasks/'+taskId+'/claim',data)}
function completeTask(taskId){return request('PUT','/tasks/'+taskId+'/complete')}
function cancelTask(taskId){return request('PUT','/tasks/'+taskId+'/cancel')}

// File
function uploadFile(tempFilePath,category='post'){
  return new Promise((resolve,reject)=>{
    const token=getToken()
    wx.uploadFile({
      url:BASE_URL+'/files/upload?category='+category,filePath:tempFilePath,name:'file',
      header:token?{'Authorization':'Bearer '+token}:{},
      success:res=>{try{resolve(JSON.parse(res.data))}catch{resolve(res.data)}},
      fail:err=>{wx.showToast({title:'上传失败',icon:'none'});reject(err)}
    })
  })
}

// Notifications
function listNotifications({cursor='',pageSize=PAGE_SIZE,type=''}={}){
  return request('GET','/notifications?'+qs({cursor,page_size:pageSize,type}))
}
function getUnreadCount(){return request('GET','/notifications/unread-count')}
function markRead(id){return request('PUT','/notifications/'+id+'/read')}
function markAllRead(){return request('PUT','/notifications/read-all')}
function deleteNotification(id){return request('DELETE','/notifications/'+id)}

// Admin
function adminBanUser(userId,reason){return request('POST','/admin/users/ban',{user_id:userId,reason})}
function adminUnbanUser(userId){return request('POST','/admin/users/unban',{user_id:userId})}
function adminListUsers(params){return request('GET','/admin/users/list?'+qs(params))}
function adminSetUserRole(targetUserId,role){return request('POST','/admin/users/set-role',{target_user_id:targetUserId,role})}
function adminListAudit(params){return request('GET','/admin/content/audit-list?'+qs(params))}
function adminAuditContent(contentId,action,reason){return request('POST','/admin/content/audit',{content_id:contentId,action,reason})}

function formatTime(unixSec){
  if(!unixSec)return '';const d=new Date(unixSec*1000);const now=new Date();const diff=now-d
  if(diff<60000)return '刚刚';if(diff<3600000)return Math.floor(diff/60000)+'分钟前'
  if(diff<86400000)return Math.floor(diff/3600000)+'小时前';if(diff<172800000)return '昨天'
  if(diff<259200000)return '前天'
  const y=d.getFullYear();const m=d.getMonth()+1;const dd=d.getDate()
  if(y===now.getFullYear())return m+'/'+dd;return y+'/'+m+'/'+dd
}

module.exports={
  request,wxLogin,getMyInfo,bindCampus,updateUserInfo,listSchools,
  listPosts,getPost,createPost,updatePost,deletePost,likePost,unlikePost,searchContent,
  listComments,listReplies,createComment,deleteComment,
  listTasks,getTask,createTask,updateTask,deleteTask,claimTask,completeTask,cancelTask,
  uploadFile,
  listNotifications,getUnreadCount,markRead,markAllRead,deleteNotification,
  adminBanUser,adminUnbanUser,adminListUsers,adminSetUserRole,adminListAudit,adminAuditContent,
  formatTime
}
