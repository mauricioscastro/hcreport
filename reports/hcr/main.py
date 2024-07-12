# from easydict import EasyDict as edict 
# import json  
import psycopg2

def define_env(env):

    @env.macro
    def hello(hello_str):
        return "Hello " + hello_str

    @env.macro
    def queryddb(query):
        print("+++")
        print(query) 
        print("+++")
        conn = psycopg2.connect(dbname="postgres", user="postgres", host="localhost")
        try:
            with conn:
                with conn.cursor() as curs:
                    curs.execute(query)
                    return curs.fetchall()
        finally:
            conn.close()
        return ()

# def on_pre_page_macros(env):
#     print("+++")
#     alternate = None
#     try:
#         alternate = env.variables.alternate
#     except:
#         pass
#     print(alternate)
#     print("+++")
#     print(env.variables.author)
#     print("+++")
#     print(env.page)
#     print("+++")  
#     print(env.config)
#     print("+++")    
#     print(env.conf.plugins['i18n'])
#     print("+++")       
#     print(json.dumps(env.conf.plugins, indent=2))
#     print("+++")        


# def on_post_page_macros(env):
#     "After macros were executed"
#     # This will add a (Markdown or HTML) footer
#     # footer = '\n'.join(
#     #     ['', '##Added Footer (Post-macro)', 'Name of the page is _%s_' % env.page.title])
#     # env.markdown += footer
#     print("+++")
#     print(env.markdown)
#     print("+++")