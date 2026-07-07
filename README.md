# iStartModel
LlamaServer大语言模型引擎的启动器，配置集中管理..

# 经验
通常模型内置的官方模板在驱动智能体时或多或少都有些问题，大抵系由于开发团队测试的环境未有大家真实环境那样多样化导致，如系千问 3.5 / 3.6 模型或以其作为基座的衍生模型请优先使用 tmpl 目录中经过真实智能体工作环境考验的 Qwen-Agentic-EN / Qwen-Agentic-HON(S/T) 模板文件，或使用模板目录的 froggeric-v21.3 千问系列模板。

如果在使用 HON(S/T) 模板期间千问系列模型出现异常，先回退到 EN 原版模板，再重复一次验证是否会二次出现同样问题。如果不再出现，则为中文模板之问题——根因系模型训练时思维链未针对中文做强化训练，否则再次出现则为模型本身有问题。

又或者系你当前使用 LlamaServer 版本有问题——由于LlamaCpp每日都在疯狂迭代，难免出错，建议长期保留一版可在你当前设备完美工作的 LlamaServer 版本作为回退时使用。

但同时，如果模型未对中文思维链做强化训练，中文模式下的思维链通常会有可能变短——止要不降质，却又可以完成原来的任务，那中文模式变相成为缩短模型过度思考的模式开关。

测试设备为 Intel Xeon E5-2696 v3 中央处理器 + 32 GB 内存 + 8 GB RTX 3060Ti 显卡 + SSD 硬盘，配置文件仅作为参数示例。

# 模型

## 修正原版且无审查模型

Qwen3.6-35B-A3B-Uncensored-Genesis 二零二六年七月六号 APEX 版：
https://huggingface.co/LuffyTheFox/Qwen3.6-35B-A3B-Uncensored-Genesis-Hermes-GGUF/tree/main

模型作者反映：原始 Qwen3.5/3.6 模型的第 0 块专家张量中存在 40% 的噪声零块，这些噪声零块仅在 Q8_0 量化下可见。三个 ssm_con1vd 张量具有巨大的大信号规模，并在整个神经网络的训练过程中累积级联误差分布。现在吹得犀兮鸠利的 Ornith ，由于亦系同样基于有缺陷的基础模型，故而当上下文窗口较大时，将触发递归生成——即死循环。

本人批注：模型作者声称通过数学方式直接修正模型的字节数据经已修复此问题，尚须长期使用验证是否真的有效。


## 推理与行动交错思维蒸馏

Qwen3.6-35B-A3B-DSV4Pro-Thinking-Distill 二零二六年六月廿六版：

https://huggingface.co/nerkyor/Qwen3.6-35B-A3B-DSV4Pro-Thinking-Distill-GGUF/tree/main

或

https://modelscope.cn/models/Merkyor/Qwen3.6-35B-A3B-DSV4Pro-Thinking-Distill-GGUF/files

模型用户反映：多数声称此模型长程任务时未遇到死循环，但个别声称此模型相对于基模在驱动智能代理时变蠢。作者回复称将下次SFT做完了要再做RL，还要重放原生代理样本防洗去元能力，再做一次针对性的DPO。

本人批注：根因可能系思维链的改变影响了模型的行动模式，进而间接避免了死循环，尚须长期使用验证是否持久稳定有效。并跟进此模型的后续更新。


## 推荐原生语言世界模型

unsloth/Qwen-AgentWorld-35B-A3B 二零二六年六月廿五 / 原版 / 推荐IQ4及以上质量：
https://huggingface.co/unsloth/Qwen-AgentWorld-35B-A3B-GGUF/tree/main

本人批注：我现在使用的主力模型，使用Qwen-Agentic-EN / Qwen-Agentic-HON(S/T) 模板，LlamaCpp引擎目前锁定在b9859。建议用于智能代理、编程场景。

mudler/Qwen-AgentWorld-35B-A3B-APEX / 原版 / 建议 APEX-I-Compact 或 APEX-Compact 及以上质量：
https://huggingface.co/mudler/Qwen-AgentWorld-35B-A3B-APEX-GGUF/tree/main

本人批注：APEX 比 UD 量化快好兮多，若稳定则可长期使用，否则建议回退到 UD 量化。