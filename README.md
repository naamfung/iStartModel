# iStartModel
LlamaServer大语言模型引擎的启动器，配置集中管理..

# 经验
通常模型内置的官方模板在驱动智能体时或多或少都有些问题，大抵系由于开发团队测试的环境未有大家真实环境那样多样化导致，如系千问 3.5 / 3.6 模型或以其作为基座的衍生模型请优先使用 tmpl 目录中经过真实智能体工作环境考验的 Qwen-Agentic-EN / Qwen-Agentic-HON(S/T) 模板文件，或使用模板目录的 froggeric-v21.3 千问系列模板。

我长期使用的是 Qwen-Agentic v2，至于 v3 是我新搞的版本，尚须时间去验证，如果不满意 v2，可以试下 v3 版本睇下效果如何。

如果在使用 HON(S/T) 模板期间千问系列模型出现异常，先回退到 EN 原版模板，再重复一次验证是否会二次出现同样问题。如果不再出现，则为中文模板之问题——根因系模型训练时思维链未针对中文做强化训练，否则再次出现则为模型本身有问题。

又或者系你当前使用 LlamaServer 版本有问题——由于LlamaCpp每日都在疯狂迭代，难免出错，建议长期保留一版可在你当前设备完美工作的 LlamaServer 版本作为回退时使用。

但同时，如果模型未对中文思维链做强化训练，中文模式下的思维链通常会有可能变短——止要不降质，却又可以完成原来的任务，那中文模式变相成为缩短模型过度思考的模式开关。

通常而言，请使用模型作者推荐的采样率。贪婪采样或极低温度采样可能会导致模型重复循环输出。

测试设备为 Intel Xeon E5-2696 v3 中央处理器 + 32 GB 内存 + 8 GB RTX 3060Ti 显卡 + SSD 硬盘，配置文件仅作为参数示例。

# 模型

## 修正原版且无审查模型

Qwen3.6-35B-A3B-Uncensored-Genesis 二零二六年七月十六号 APEX 版：
https://huggingface.co/LuffyTheFox/Qwen3.6-35B-A3B-Uncensored-Genesis-Hermes-V3-GGUF/tree/main

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

本人批注：我现在使用的主力模型，使用Qwen-Agentic-EN / Qwen-Agentic-HON(S/T) 模板，LlamaCpp引擎目前使用我自己维护的[稳定分支](https://github.com/naamfung/laamaafung)。建议用于智能代理、编程场景。

mudler/Qwen-AgentWorld-35B-A3B-APEX / 原版 / 建议 APEX-I-Compact 或 APEX-Compact 及以上质量：
https://huggingface.co/mudler/Qwen-AgentWorld-35B-A3B-APEX-GGUF/tree/main

本人批注：APEX 比 UD 量化快好兮多，若稳定则可长期使用，否则建议回退到 UD 量化。


# 问题

## 思考后停止

自回归模型天然存在死循环的可能性，止系概率或大或小而已，但这其实并不致命，无论模型的训练质量如何，我们前端用户至少可以通过调用方代理程序的内部逻辑来监控模型是否在死循环，之后再以各种方式去处理或避免此问题的影响。

但接下来的情况就比较令人困扰了……

https://github.com/ggml-org/llama.cpp/issues/20260

交错式思维或者讲交错推理模型会有高概率触发「思考后正准备行动却停止响应」，或者通俗地讲就系时常自行停止继续工作。

所有倾向于在思考块中添加工具调用的模型都会有可能触发此错误，频率或高或低，止视乎模型何时于思考期间调用工具，不过问题不在于模型，而在于推理引擎——事情的发生可能来自LlamaCpp b8227版本引入的新解析器，然而 b8255 时就有用户遇到这个问题了，所以回滚到之前并不会解决问题，甚至有可能更坏。

更令人头痛的系，触发此错误具有一定的随机性，有些版本较少遇到此问题，而有些却总是遇到，介于两者之间的版本有时能够完成请求，但通常都会失败。

冲突之处：当解析器遇到“文本前缀 + 工具标签”的组合时，会因为无法匹配预设的格式而报错（如 Failed to parse input at pos N）。在流式传输中，这可能导致整个请求失败，表现为模型“停止响应”。

引发问题的 PEG 解析器具体输入案例，通常类似：「让我再查看一些额外的信息来完善分析。\n\n<tool_call>...」紧随其后就系自动停止响应。

模型输出的“思维链”内容与“工具调用”指令在格式上混在一齐，而 LlamaCpp 的解析器难以准确区分二者，导致解析失败进而停止响应。

因为语法触发器独立于推理解析器，并且语法 eog 标签会在读取完佢认为是工具调用的内容后强制终止，因此模型无法继续生成——佢应关闭思考块之后，再在正常内容块中正确地执行工具调用。

讲到底就系大语言模型的发展如烈火烹油，模型迭代太快了，而作为推理引擎的 LlamaCpp 却未及时支持越来越复杂的“思维链”与“工具调用”模型，其解析器架构未跟上模型行为演变所导致。

再举个例，DeepSeek 官方模型会在思考期间自动调用搜索工具得到搜索结果继而进一步推演的行为就可以精确描述并证明模型在思考期间调用工具的正当性。

所以，问题的根源在于解析器的僵化逻辑与模型灵活的输出方式之间的矛盾。

然而 LlamaCpp 的部份维护者初期并未意识到这是模型的正常行为，甚至认为模型不应该在推理块中输出工具调用。

开口 think 标签是在预填充中注入的，但有时模型未将其关闭——这不是模型的错误，模型训练时好可能就是要在思考期间亦支持调用工具，继而导致 LlamaCpp 认为模型仍在“思考”，接下来又导致 tool_call 标签无法被识别与调度。

据传曾经有效的临时解决方案：「--reasoning-format=none」，「--reasoning-budget 0」或止用「--reasoning off」等方式关闭思考。且对于并行工具调用，必须传入parallel_tool_calls: true请求。

可以尝试不确保正常的特定版本：b8683 b9654，且 b9655 至到 b9694 乃至最新版本都有用户报告思考出现多工具调用时因解析出错而停止工作。

自行试用或等此方案合并 https://github.com/ggml-org/llama.cpp/pull/24202

我喊 Klaude Code 帮做了合并，但经我验证，#24202 方案并未解决问题。

于是有了以下我自己维护的稳定分支，当前 MASTER 分支经已长久未再遇到以上无故停止的生产事故，
但如果你使用了我的 MASTER 分支仍然遇到此类问题，建议在 LlamaServer 的启动参数中加上「 --verbose --verbosity 5」以截获无故停止工作时的关键错误，然后提交日志到我这个[稳定分支](https://github.com/naamfung/laamaafung)。


由于 LlamaCpp 项目所有发布的版本序号单纯就系提交次数累加却非真正的生产就绪，故而，此分支的唯一目标是成为一个可驱动智能代理正常工作的稳定版本。


克隆或下载：

```sh
git clone --depth 1 https://github.com/naamfung/laamaafung.git
```

配置与编译：

```sh
cmake -B build -DGGML_CUDA=ON -DGGML_NATIVE=ON -DGGML_CUDA_FA=ON -DGGML_CUDA_FA_ALL_QUANTS=ON -DCMAKE_BUILD_TYPE=Release

cmake --build build -j --config Release
```

如果条件允许，另一种更现实的方案系更换推理引擎：建议换 vllm 或 sglang 试下。

# 思考

对于混合模型而言，思考过程为必须品，因为思考过程会将所有可能性预先触达一次，以便加载到键值缓存中，而键值缓存中是否存在相关上下文数据就是下阶段生成时的燃料。
