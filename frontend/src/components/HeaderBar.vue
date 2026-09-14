<script setup>
import { ref, onMounted } from 'vue'
import { Minus, FullScreen, Close } from '@element-plus/icons-vue'
import { WindowMinimise, WindowToggleMaximise, WindowClose, GetProcessName } from '../../wailsjs/go/service/App'
import { useI18n } from '../composables/useI18n'

const { t } = useI18n()
const appName = ref('')

onMounted(async () => {
  try {
    appName.value = await GetProcessName()
  } catch (e) {
    appName.value = t('app_name', 'wails-temp')
  }
})
</script>

<template>
  <!-- 上边栏 -->
  <div class="top-bar" @dblclick="WindowToggleMaximise">
    <!-- 左侧 Logo 和标题 -->
    <div class="left-panel">
      <span class="app-title">{{ appName }}</span>
    </div>

    <!-- 右侧窗口控制按钮 -->
    <div class="right-panel">
      <button class="window-btn" @click="WindowMinimise">
        <el-icon><Minus /></el-icon>
      </button>
      <button class="window-btn" @click="WindowToggleMaximise">
        <el-icon><FullScreen /></el-icon>
      </button>
      <button class="window-btn close-btn" @click="WindowClose">
        <el-icon><Close /></el-icon>
      </button>
    </div>
  </div>
</template>

<style scoped>
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  height: 30px;
  background-color: #fff;
  border-bottom: 1px solid #e4e7ed;
  --wails-draggable: drag;
  user-select: none;
}

.left-panel {
  display: flex;
  align-items: center;
}

.app-title {
  font-weight: 600;
  font-size: 13px;
  color: #303133;
}

.right-panel {
  display: flex;
  align-items: center;
  gap: 4px;
  -webkit-app-region: no-drag;
}

.window-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  border: none;
  background-color: transparent;
  color: #606266;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.2s ease;
}

.window-btn:hover {
  background-color: rgba(0, 0, 0, 0.08);
}

.window-btn:active {
  background-color: rgba(0, 0, 0, 0.12);
}

.close-btn:hover {
  background-color: #f56c6c;
  color: #fff;
}

.close-btn:active {
  background-color: #e64242;
}
</style>