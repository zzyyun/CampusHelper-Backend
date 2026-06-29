const api=require('../../utils/api')
const {POST_TYPE_TEXT,POST_TYPE_TAG,CONDITION_TEXT,TRADE_METHOD_TEXT,LOST_OR_FOUND_TEXT,ITEM_CATEGORY_TEXT}=require('../../utils/constants')
Page({
  data:{post:null,isLiked:false,isOwner:false,comments:[],cursor:'',hasMore:true,loading:false,commentText:'',replyTo:null,
    postTypeText:POST_TYPE_TEXT,postTypeTag:POST_TYPE_TAG,conditionText:CONDITION_TEXT,tradeMethodText:TRADE_METHOD_TEXT,lostFoundText:LOST_OR_FOUND_TEXT,categoryText:ITEM_CATEGORY_TEXT},
  onLoad(o){if(o.id){this.data.postId=o.id;this.loadPost();this.loadComments()}},
  loadPost(){api.getPost(this.data.postId).then(d=>{this.setData({post:d.post,isLiked:d.is_liked,isOwner:d.is_owner});wx.setNavigationBarTitle({title:d.post.title||'帖子详情'})}).catch(()=>wx.showToast({title:'加载失败',icon:'none'}))},
  loadComments(r){if(this.data.loading)return;this.setData({loading:true})
    api.listComments(this.data.postId,r?'':this.data.cursor).then(d=>{this.setData({comments:r?(d.comments||[]):this.data.comments.concat(d.comments||[]),cursor:d.next_cursor||'',hasMore:d.has_more,loading:false})}).catch(()=>this.setData({loading:false}))},
  onLoadMore(){if(this.data.hasMore&&!this.data.loading)this.loadComments()},
  onToggleLike(){(this.data.isLiked?api.unlikePost:api.likePost)(this.data.postId).then(d=>this.setData({isLiked:d.liked,'post.likes_count':d.likes_count})).catch(()=>wx.showToast({title:'操作失败',icon:'none'}))},
  onDelete(){wx.showModal({title:'确认删除',content:'确定要删除此帖子吗？',success:r=>{if(r.confirm){api.deletePost(this.data.postId).then(()=>{wx.showToast({title:'已删除',icon:'success'});wx.navigateBack()}).catch(()=>wx.showToast({title:'删除失败',icon:'none'}))}}})},
  onCommentInput(e){this.setData({commentText:e.detail.value})},
  onReplyTo(e){this.setData({replyTo:e.currentTarget.dataset.comment})},
  cancelReply(){this.setData({replyTo:null})},
  onSubmitComment(){const c=this.data.commentText.trim();if(!c)return wx.showToast({title:'请输入评论内容',icon:'none'})
    api.createComment({post_id:parseInt(this.data.postId),content:c,parent_id:this.data.replyTo?this.data.replyTo.id:0}).then(()=>{this.setData({commentText:'',replyTo:null});this.data.cursor='';this.loadComments(true);wx.showToast({title:'评论成功',icon:'success'})}).catch(()=>wx.showToast({title:'评论失败',icon:'none'}))},
  onDeleteComment(e){const id=e.currentTarget.dataset.id;wx.showModal({title:'确认删除',content:'确定要删除此评论吗？',success:r=>{if(r.confirm){api.deleteComment(id).then(()=>{this.data.cursor='';this.loadComments(true)}).catch(()=>wx.showToast({title:'删除失败',icon:'none'}))}}})},
  onImagePreview(e){wx.previewImage({urls:e.currentTarget.dataset.urls||[],current:e.currentTarget.dataset.src||''})}
})
