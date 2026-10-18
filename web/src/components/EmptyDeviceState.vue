<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  detail?: string
}>(), {
  title: '未检测到兼容 QMI 设备',
  detail: '当前服务运行在 no-device 模式，不会尝试打开、映射或修改任何 modem 节点。'
})
</script>

<template>
  <section class="empty-device-state" role="status">
    <span class="empty-device-icon" aria-hidden="true">○</span>
    <div>
      <h2>{{ title }}</h2>
      <p>{{ detail }}</p>
      <p class="muted">请确认设备已由外部流程完成兼容配置；如果该节点被 VoHive 使用，请保持 QMI Web 的 no-device 部署。</p>
    </div>
  </section>
</template>
