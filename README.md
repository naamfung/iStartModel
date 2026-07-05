# iStartModel
LlamaServer大语言模型引擎的启动器，配置集中管理..

# 经验
通常模型内置的官方模板在驱动智能体时或多或少都有些问题，大抵系由于开发团队测试的环境未有大家真实环境那样多样化导致，如系千问 3.5 / 3.6 模型或以其作为基座的衍生模型请优先使用 tmpl 目录中经过真实智能体工作环境考验的 Qwen-Agentic-EN / Qwen-Agentic-HON(S/T) 模板文件，或使用模板目录的 froggeric-v21.3 千问系列模板。

我的设备为 Intel Xeon E5-2696 v3 中央处理器 + 32 GB 内存 + 8 GB RTX 3060Ti 显卡 + SSD 硬盘，配置文件仅作为参数示例，尤其速度数据系基于我自己的设备。

# 模型

推荐无审查模型

Qwen3.6-35B-A3B-Uncensored-Genesis 二零二六年七月二号版：
https://huggingface.co/LuffyTheFox/Qwen3.6-35B-A3B-Uncensored-Genesis-GGUF/tree/main


强烈推荐原生语言世界模型

Qwen-AgentWorld-35B-A3B 二零二六年六月廿五版：
https://huggingface.co/unsloth/Qwen-AgentWorld-35B-A3B-GGUF/tree/main
