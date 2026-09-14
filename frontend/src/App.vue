<script setup>
import { ref } from 'vue'
import Test from './components/Test.vue'
import HeaderBar from './components/HeaderBar.vue'
import LangSelector from './components/LangSelector.vue'
import { useI18n } from './composables/useI18n'

const { t } = useI18n()
const activeTab = ref('features')
</script>


<template>
  <div class="app-layout">
    <HeaderBar />
    <div class="app-content">
      <el-tabs v-model="activeTab" class="main-tabs" stretch>
        <el-tab-pane :label="t('feature_overview', '功能概览')" name="features">
          <Test />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped>
.app-layout {
  width: 100vw;
  height: 100vh;
  display: flex;
  flex-direction: column;
  background-color: #f5f7fa;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  user-select: none;
}

.app-content {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px;
}

.header-controls {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.main-tabs {
  min-height: 100%;
}

.main-tabs :deep(.el-tabs__header) {
  margin: 0;
  padding: 8px 0;
}

.main-tabs :deep(.el-tabs__content) {
  padding: 0;
}

.main-tabs :deep(.el-tab-pane) {
  min-height: 100%;
}
</style>
